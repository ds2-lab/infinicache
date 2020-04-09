package global_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/neboduus/infinicache/proxy/proxy/global"
)

func TestStorage(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Global")
}

var _ = Describe("IP", func() {
	It("should find private ip", func() {
		_, err := global.GetPrivateIp()
		Expect(err).To(BeNil())
	})
})
