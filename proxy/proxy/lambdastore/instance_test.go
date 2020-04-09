package lambdastore_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"


	"github.com/neboduus/infinicache/proxy/proxy/lambdastore"
)

var _ = Describe("Instance", func() {
	It("should Switch() change id and name.", func() {
		ins := lambdastore.NewInstanceFromDeployment(lambdastore.NewDeployment("Inst", 1, false))
		deploy := lambdastore.NewDeployment("Inst", 2, false)

		ins.Switch(deploy)

		Expect(ins.Id()).To(Equal(uint64(2)))
		Expect(ins.Name()).To(Equal("Inst2"))
		Expect(deploy.Id()).To(Equal(uint64(1)))
		Expect(deploy.Name()).To(Equal("Inst1"))
	})

})
