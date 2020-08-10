// 尚未解决/有可能出现的问题
// 1. 如果中间有断条,重启时有可能将当天已建立索引的数据重复建索引.所以如果程序意外退出,建议定时0:00重启.

package main

import (
	"github.com/erikdubbelboer/gspt"

	"nginx2es/pkg/work"
)

const programName = "nginx2es"

func main() {
	gspt.SetProcTitle(programName)

	work.Work()
}
