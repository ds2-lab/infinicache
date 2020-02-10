# Evaluation

# deployment

AMI: ubuntu-xenial-16.04
Golang: 1.12

~~~
sudo add-apt-repository ppa:longsleep/golang-backports
sudo apt-get update
sudo apt-get -y upgrade
sudo apt-get install golang-go

echo "export GOPATH=$HOME/go" >> $HOME/.bashrc
. $HOME/.bashrc
mkdir -p $HOME/go/src/github.com/wangaoone
cd $HOME/go/src/github.com/wangaoone
git clone https://github.com/wangaoone/LambdaObjectstore.git LambdaObjectstore
git clone https://github.com/mason-leap-lab/redeo.git redeo
git clone https://github.com/wangaoone/redbench.git redbench

cd LambdaObjectstore/
git checkout config/[tianium]
git pull
cd src
go get

cd $HOME/go/src/github.com/wangaoone/redbench
go get

sudo apt install awscli
cd $HOME/go/src/github.com/wangaoone/LambdaObjectstore/evaluation
make deploy
~~~
