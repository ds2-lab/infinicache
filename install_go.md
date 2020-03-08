# Golang Install

You can refer to this article about how to install Golang on [Ubuntu AMI](https://tecadmin.net/install-go-on-ubuntu/) and [Amazon AMI](https://hackernoon.com/deploying-a-go-application-on-aws-ec2-76390c09c2c5).

Also, in current release, we recommend to use **Golang 1.12** and **Ubuntu AMI** on AWS.

```bash
sudo apt-get update
sudo apt-get -y upgrade
```

Download the Go language binary archive.

```bash
wget https://dl.google.com/go/go1.12.linux-amd64.tar.gz
sudo tar -xvf go1.12.linux-amd64.tar.gz
sudo mv go /usr/local
```

Setup Go environment, including `GOROOT` and `GOPATH`. Add enviornment variables to the `~/.profile`.

```bash
export GOROOT=/usr/local/go
mkdir $HOME/project
export GOPATH=$HOME/project
export PATH=$GOPATH/bin:$GOROOT/bin:$PATH
```

Verify installation

```bash
~$ go version
~$ go version go1.12 linux/amd64

~$ go env
...
GOOS="linux"
GOPATH="/home/ubuntu/project"
GOPROXY=""
GORACE=""
GOROOT="/usr/local/go"
...
```

If you could see above information, you are all set with Golang installation!