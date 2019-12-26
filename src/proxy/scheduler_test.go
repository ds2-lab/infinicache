package proxy

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

//	"github.com/wangaoone/LambdaObjectstore/src/proxy"
)

var _ = Describe("Instance", func() {
	var scheduler *Scheduler

	setup := func() {
		scheduler = NewScheduler(2, 5)
	}

	It("should instance fetch return switched instance on migrate.", func() {
		setup()

		group := NewGroup(2)
		ins1 := scheduler.GetForGroup(group, 0)
		dpl2, _ := scheduler.ReserveForGroup(group, 0)
		ins11, _ := scheduler.Instance(dpl2.Id())

		Expect(ins11).To(Equal(ins1))
		Expect(ins11.Id()).To(Equal(uint64(1)))
		Expect(dpl2.Id()).To(Equal(uint64(0)))

		// fixed: delete keys in scheduler.actives wrongly
		ins12, _ := scheduler.Instance(ins1.Id())

		Expect(ins12).To(Equal(ins1))
		Expect(ins12.Id()).To(Equal(uint64(1)))
	})

})
