package lambdastore_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestLambdastore(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Lambdastore")
}
