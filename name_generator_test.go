package wasabi_test

import (
	"github.com/cloudkucooland/WASABI"
	"testing"
)

func BenchmarkGenerateName(b *testing.B) {
	for i := 0; i < b.N; i++ {
		wasabi.GenerateName()
	}
}

func BenchmarkGenerateSafeName(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := wasabi.GenerateSafeName()
		if err != nil {
			b.Error(err.Error())
		}
	}
}
