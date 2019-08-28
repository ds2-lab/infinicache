package storage_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/wangaoone/LambdaObjectstore/src/LambdaStore/storage"
)

func TestStorage(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Storage")
}

var _ = Describe("Storage", func() {
	var store *storage.Storage

	setup := func() {
		store = storage.New()
		store.Set("key1", "1", nil)
		store.Set("key2", "1", nil)
	}

	It("should Keys() return all keys in latest first order.", func() {
		setup()

		keys := store.Keys()
		Expect(store.Len()).To(Equal(2))
		Expect(<-keys).To(Equal("key2"))
		Expect(<-keys).To(Equal("key1"))
	})

})
