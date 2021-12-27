package wasabee_test

import (
	"github.com/wasabee-project/Wasabee-Server"
	"testing"
)

func BenchmarkGenerateName(b *testing.B) {
	for i := 0; i < b.N; i++ {
		wasabee.GenerateName()
	}
}

func BenchmarkGenerateSafeName(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := wasabee.GenerateSafeName()
		if err != nil {
			b.Error(err.Error())
		}
	}
}
