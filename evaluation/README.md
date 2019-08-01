# Evaluation

# deployment

AMI: ubuntu-xenial-16.04
install go as in: https://tecadmin.net/install-go-on-ubuntu/
go version: 1.11.12

export GOPATH=$HOME/go
mkdir -p $HOME/go/src/github.com/wangaoone
cd $HOME/go/src/github.com/wangaoone
git clone https://github.com/wangaoone/LambdaObjectstore.git LambdaObjectstore
git clone https://github.com/wangaoone/redeo.git redeo
git clone https://github.com/wangaoone/ecRedis.git ecRedis
git clone https://github.com/wangaoone/redbench.git redbench

cd ../LambdaObjectstore/
git checkout config/[tianium]
git pull
cd ../src
go get

cd $HOME/go/src/github.com/wangaoone/redbench
go get

sudo apt install awscli
# sudo apt install python3-pip

cd $HOME/go/src/github.com/wangaoone/LambdaObjectstore/evaluation
