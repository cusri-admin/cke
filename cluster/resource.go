package cluster

//Resource 描述资源使用量的struct
type Resource struct {
	CPU  float64 `json:"cpu"`
	Mem  float64 `json:"mem"`
	Disk float64 `json:"disk"`
}

//AddResource 对资源进行累加
func (res *Resource) AddResource(r *Resource) {
	if r != nil {
		res.CPU += r.CPU
		res.Mem += r.Mem
		res.Disk += r.Disk
	}
}

//Set 赋值
func (res *Resource) Set(r *Resource) {
	res.CPU = r.CPU
	res.Mem = r.Mem
	res.Disk = r.Disk
}