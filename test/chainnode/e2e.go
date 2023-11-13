package chainnode

import (
	"github.com/NibiruChain/nibiru-operator/test/framework"
)

var tf *framework.TestFramework

func RegisterTestFramework(testFramework *framework.TestFramework) {
	tf = testFramework
}
