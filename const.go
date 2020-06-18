package ark

const (
	DEFAULT = "default"
	SERVICE = "service"
	GATEWAY = "gateway"

	BROWSE_TOKEN  = 0
	PREVIEW_TOKEN = 1
)

const (
	UTF8   = "utf-8"
	GB2312 = "gb2312"
	GBK    = "gbk"
)

const (
	DataCreateTrigger  = "$.data.create"
	DataChangeTrigger  = "$.data.change"
	DataRemoveTrigger  = "$.data.remove"
	DataRecoverTrigger = "$.data.recover"

	StartTrigger = "$.ark.start"
	StopTrigger  = "$.ark.stop"
)
