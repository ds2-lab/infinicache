package migrator

import (
	"fmt"
	"encoding/json"
	"errors"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/wangaoone/LambdaObjectstore/lib/logger"
	"github.com/wangaoone/redeo"
	"github.com/wangaoone/redeo/resp"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/wangaoone/LambdaObjectstore/src/LambdaStore/types"
)

var (
	log = &logger.ColorLogger{
		Level: logger.LOG_LEVEL_WARN,
	}
	MigrationTimeout = 30 * time.Second
	ErrClosedPrematurely = errors.New("Client closed before ready.")
)

type Client struct {
	addr         string
	cn           net.Conn
	ready        chan error
	mu           sync.Mutex
	w            *resp.RequestWriter
	r            resp.ResponseReader
}

func NewClient() *Client {
	return &Client{
		ready: make(chan error, 1), // We don't want to block the channel.
	}
}

func (cli *Client) Initiate(initiator func() error) error {
	err := initiator()
	if err != nil {
		return err
	}

	// Test ready and reset if neccessary
	select{
	case err := <-cli.ready:
		if err == nil {
			// closed, reopen
			cli.ready = make(chan error, 1)
		}
	default:
	}

	return <-cli.ready
}

func (cli *Client) Connect(addr string) (err error) {
	cli.cn, err = net.Dial("tcp", addr)
	if err != nil {
		cli.ready <- err
		return
	}

	// FIXME: Time out not working.
	cli.cn.SetDeadline(time.Now().Add(MigrationTimeout))
	return
}

func (cli *Client) TriggerDestination(dest string, args interface{}) (err error) {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	client := lambda.New(sess, &aws.Config{Region: aws.String("us-east-1")})
	payload, _ := json.Marshal(args)
	input := &lambda.InvokeInput{
		FunctionName:   aws.String(dest),
		Payload:        payload,
		InvocationType: aws.String("Event"), /* async invoke*/
	}

	res, err := client.Invoke(input)
	if err == nil && *res.StatusCode >= 300 {
		err = errors.New(fmt.Sprintf("Unexpected http code on triggering destination of migration: %d", *res.StatusCode))
	}
	if err != nil {
		cli.ready <- err
	}
	return
}

func (cli *Client) Send(cmd string, args ...string) (resp.ResponseReader, error) {
	if cli.w == nil && cli.r == nil {
		cli.w = resp.NewRequestWriter(cli.cn)
		cli.r = resp.NewResponseReader(cli.cn)
	}

	// init backup cmd
	cli.w.WriteCmdString(cmd, args...)
	if err := cli.w.Flush(); err != nil {
		return nil, err
	}

	return cli.r, nil
}

func (cli *Client) WaitForMigration(srv *redeo.Server) {
	defer cli.Close()

	err := srv.ServeForeignClient(cli.cn)
	if err == nil {
		return
	} else if !cli.IsReady() {
		if err == io.EOF {
			err = ErrClosedPrematurely
		}
		log.Warn("Migration connection closed: %v", err)
		cli.ready <- err
	} else if err != io.EOF {
		log.Warn("Migration connection closed: %v", err)
	} else {
		log.Info("Migration Connection closed.")
	}
}

func (cli *Client) Migrate(reader resp.ResponseReader, store types.Storage) {
	defer cli.Close()

	reader.ReadBulkString() // skip command
	strLen, err := reader.ReadBulkString()
	len := 0
	if err != nil {
		log.Error("Failed to read length of data from lambda: %v", err)
		return
	} else {
		len, err = strconv.Atoi(strLen)
		if err != nil {
			log.Error("Convert strLen err: %v", err)
			return
		}
	}

	keys := make([]string, len)
	for i := 0; i < len; i++ {
		keys[i], err = reader.ReadBulkString()
		if err != nil {
			log.Error("Failed to read migration keys: %v", err)
			return
		}
	}

	// Start migration
	log.Debug("Start migrating %d keys", len)
	for _, key := range keys {
		err := store.(*StorageAdapter).Migrate(key)
		if err == ErrSkip {
			log.Debug("Migrating key %s: %v", key, err)
		} else if err != nil {
			log.Warn("Migrating key %s: %v", key, err)
		} else {
			log.Debug("Migrating key %s: success", key)
		}
	}
	log.Debug("End migration")
}

func (cli *Client) SetError(err error) {
	cli.ready <- err
}

func (cli *Client) SetReady() {
	select {
	case <-cli.ready:
	default:
	}
	
	close(cli.ready)
}

func (cli *Client) Ready() <-chan error {
	return cli.ready
}

func (cli *Client) IsReady() bool {
	select {
	case err := <-cli.ready:
		if err == nil {
			return true
		} else {
			cli.ready <- err
			return false
		}
	default:
		return false
	}
}

func (cli *Client) GetStoreAdapter(store types.Storage) *StorageAdapter {
	return newStorageAdapter(cli, store)
}

func (cli *Client) Close() {
	cli.cn.Close()
}
