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
	"sync"
)

var log = &logger.ColorLogger{
	Level: logger.LOG_LEVEL_WARN,
}

type Client struct {
	addr         string
	cn           net.Conn
	ready        chan error
	mu           sync.Mutex
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

	return <-cli.ready
}

func (cli *Client) Connect(addr string) (err error) {
	cli.cn, err = net.Dial("tcp", addr)
	if err != nil {
		cli.ready <- err
	}
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

func (cli *Client) Send(cmd string, args ...string) error {
	writer := resp.NewRequestWriter(cli.cn)
	// init backup cmd
	writer.WriteCmdString(cmd, args...)
	return writer.Flush()
}

func (cli *Client) Start(srv *redeo.Server) {
	err := srv.ServeForeignClient(cli.cn)
	if err != nil && err != io.EOF {
		log.Warn("Migration connection closed: %v", err)
	} else {
		log.Info("Migration Connection closed.")
	}
}

func (cli *Client) SetError(err error) {
	cli.ready <- err
}

func (cli *Client) SetReady() {
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
