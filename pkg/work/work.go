package work

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/astaxie/beego/config"
	"github.com/astaxie/beego/logs"
	"github.com/hpcloud/tail"
	"github.com/olivere/elastic/v7"

	"nginx2es/pkg/tools"
)

var bizLogMappings = `
{
    "mappings":{
        "properties":{
            "@timestamp" : {
                 "type" : "date"
            },
            "@version" : {
              "type" : "integer"
            }
        }
    }
}
`

// Delete data from a few months ago
const DeleteBefore int64 = 3

type WorkerConf struct {
	Project       string
	ListenFiles   []string
	EsHost        string
	EsIndexPrefix string
	Hostname      string
	Exclude       []string
}

var (
	workConf = WorkerConf{}
	cfg      config.Configer
)

func init() {
	var err error
	cfg, err = config.NewConfig("ini", "conf/app.conf")
	if err != nil {
		panic(fmt.Sprintf("can not parse config file: conf/app.conf, %v\n", err))
	}

	workConf.ListenFiles = make([]string, 0)
	workConf.Hostname = tools.Hostname()

	var logFileBox []string
	logFileJson := cfg.String("logs")
	err = json.Unmarshal([]byte(logFileJson), &logFileBox)
	if err != nil {
		panic(fmt.Sprintf("can not json decode logs. err: %v\n", err))
	}
	if len(logFileBox) <= 0 {
		panic(`no log file config`)
	}

	for _, file := range logFileBox {
		workConf.ListenFiles = append(workConf.ListenFiles, file)
	}

	workConf.Project = cfg.String("project")
	workConf.EsHost = cfg.String("es_host")
	workConf.EsIndexPrefix = cfg.String("es_index_prefix")

	if workConf.Project == "" ||
		len(workConf.ListenFiles) == 0 || workConf.EsHost == "" ||
		workConf.EsIndexPrefix == "" {
		panic("wrong config, please checkout")
	}

	logs.Debug("[work.init] workConf: %#v", workConf)
}

func Work() {
	for {
		mainEsCline, _ := newEsClient(workConf.EsHost)
		// 每轮都尝试轮循删除三个月的索引
		tryDeleteIndex(mainEsCline, DeleteBefore)

		// 尝试创建当月索引的mappings
		tryBuildCurrentMappings(mainEsCline)

		var wg sync.WaitGroup

		// 监听文件变化,并创建es索引数据
		for _, logFilename := range workConf.ListenFiles {
			wg.Add(1)
			go tailFile4Es(&wg, logFilename)
		}

		// 主 goroutine,等待工作 goroutine 正常结束
		wg.Wait()

		// 休眠一下下,给其他进程切换日志的时间
		time.Sleep(5 * time.Minute)
	}
}

func tryDeleteIndex(client *elastic.Client, before int64) {
	dayTime := tools.NaturalDay(-(before * 30))
	esIndex := fmt.Sprintf(`%s%s`, workConf.EsIndexPrefix, tools.LocalYearMonth(dayTime))

	//logs.Debug("[tryDeleteIndex] esIndex:", esIndex)

	yes, errE := client.IndexExists(esIndex).Do(context.Background())
	if yes {
		result, errD := client.DeleteIndex(esIndex).Do(context.Background())
		if errD != nil {
			logs.Error("[tryDeleteIndex] delete index fail, esIndex:", esIndex, ", result:", result, ", errD:", errD)
		} else {
			logs.Informational("[tryDeleteIndex] delete index success, esIndex:", esIndex)
		}
	} else {
		logs.Informational("[tryDeleteIndex] no index found, index: %s, errE: %v", esIndex, errE)
	}
}

func tryBuildCurrentMappings(client *elastic.Client) {
	dayTime := tools.NaturalDay(0)
	esIndex := fmt.Sprintf(`%s%s`, workConf.EsIndexPrefix, tools.LocalYearMonth(dayTime))

	logs.Debug("[tryBuildCurrentMappings] esIndex:", esIndex)

	yes, _ := client.IndexExists(esIndex).Do(context.Background())
	if yes {
		logs.Informational("[tryBuildCurrentMappings] esIndex: %s has exsit", esIndex)
	} else {
		result, errC := client.CreateIndex(esIndex).Body(bizLogMappings).Do(context.Background())
		if errC != nil {
			logMsg := fmt.Sprintf("[tryBuildCurrentMappings] can NOT set mapppings for esIndex: %s, err: %v", esIndex, errC)
			panic(logMsg)
		} else {
			logs.Informational("[tryBuildCurrentMappings] set mappings successful for esIndex:", esIndex, ", result:", result)
		}
	}
}

func tailFile4Es(wg *sync.WaitGroup, logFilename string) {
	defer wg.Done()

	logs.Informational("[tailFile4Es] tail -f %s", logFilename)

	// 如果是5:00以后重启程序,那么跳过之前的日志,避免重启导入旧日志,但有可能丢失部分日志,暂时不考虑了... {
	tfConf := tail.Config{
		Follow:    true,
		ReOpen:    true,
		MustExist: true,
	}
	topT := time.Now()
	topHour := topT.Hour()
	if topHour >= 5 {
		fileInfo, err := os.Stat(logFilename)
		if err != nil {
			logs.Error("[tailFile4Es] stat file has wrong, logFilename: %s, err: %v", logFilename, err)
			return
		}

		tfConf.Location = &tail.SeekInfo{
			Offset: fileInfo.Size(),
			Whence: io.SeekStart,
		}
		// 下面的方法也有效,并且节省一次文件操作开销
		// &tail.SeekInfo{Offset: 0, Whence: io.SeekEnd}
	}
	// end }

	tf, err := tail.TailFile(logFilename, tfConf)
	if err != nil {
		logs.Error("[tailFile4Es] tail -f has wrong, err:", err)
		return
	}

	workEsCline, _ := newEsClient(workConf.EsHost)

	var checked = false
	for line := range tf.Lines {
		// // 00:00 切换日志,工作进程重启,主协程会休眠,可以错开时间,防止无限重复事件
		// t := time.Now()
		// hour := t.Hour()
		// minute := t.Minute()
		// if hour == 0 && minute == 0 {
		// 	logs.Informational("[tailFile4Es] it is time to logrotate. logFilename:", logFilename)
		// 	break
		// }

		dayTime := tools.NaturalDay(0)
		esIndex := fmt.Sprintf(`%s%s`, workConf.EsIndexPrefix, tools.LocalYearMonth(dayTime))

		now := time.Now()
		if now.Day() == 1 {
			if !checked {
				tryDeleteIndex(workEsCline, DeleteBefore)
				tryBuildCurrentMappings(workEsCline)
				checked = true

				logs.Informational("[tailFile4Es] need create new index mappings and delete old index, day: %d, esIndex: %s, logFilename: %s", now.Day(), esIndex, logFilename)
			}
		} else {
			checked = false
		}

		var ignore bool
		for _, exclude := range workConf.Exclude {
			if strings.HasSuffix(line.Text, exclude) {
				ignore = true
				break
			}
		}
		if ignore {
			continue
		}

		_, errD := workEsCline.Index().
			Index(esIndex).
			BodyJson(line.Text).
			Do(context.Background())
		if errD != nil {
			logs.Error("[tailFile4Es] es do exception, err: %v", errD)
		}

		logs.Debug("[tailFile4Es] origin log: %v", line.Text)
	}

	return
}

func newEsClient(esHost string) (client *elastic.Client, err error) {
	client, err = elastic.NewClient(elastic.SetURL(esHost))

	if err != nil {
		logs.Emergency("[newEsClient] can NOT new es client, esHost:", esHost, "err:", err)
		panic(err)
	}

	return
}
