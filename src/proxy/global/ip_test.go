package global

import (
	// "testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("IP", func() {
	It("should find private ip", func() {
		_, err := GetPrivateIp()
		Expect(err).To(BeNil())
	})
})
