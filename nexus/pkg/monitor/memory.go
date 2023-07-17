package monitor

// import (
// 	"github.com/shirou/gopsutil/v3/mem"
// 	"github.com/shirou/gopsutil/v3/process"
// )
//
// type Sample struct {
// 	samples []float64
// }
//
// func (s *Sample) clear() {
// 	s.samples = []float64{}
// }
//
// type ProcessMemoryRSS struct {
// 	Sample
// 	pid     int32
// 	process *process.Process
// 	name    string
// }
//
// func NewProcessMemoryRSS(pid int32) *ProcessMemoryRSS {
// 	return &ProcessMemoryRSS{
// 		pid:  pid,
// 		name: "proc.memory.rssMB",
// 	}
// }
//
// func (p *ProcessMemoryRSS) sample() {
// 	if p.process == nil {
// 		p.process, _ = process.NewProcess(p.pid)
// 	}
// 	memInfo, _ := p.process.MemoryInfo()
// 	p.samples = append(p.samples, float64(memInfo.RSS)/1024/1024)
// }
//
// type ProcessMemoryPercent struct {
// 	Sample
// 	pid     int32
// 	process *process.Process
// 	name    string
// }
//
// func NewProcessMemoryPercent(pid int32) *ProcessMemoryPercent {
// 	return &ProcessMemoryPercent{
// 		pid:  pid,
// 		name: "proc.memory.percent",
// 	}
// }
//
// func (p *ProcessMemoryPercent) sample() {
// 	if p.process == nil {
// 		p.process, _ = process.NewProcess(p.pid)
// 	}
// 	memPercent, _ := p.process.MemoryPercent()
// 	p.samples = append(p.samples, float64(memPercent))
// }
//
// type MemoryPercent struct {
// 	Sample
// 	name string
// }
//
// func NewMemoryPercent() *MemoryPercent {
// 	return &MemoryPercent{
// 		name: "memory",
// 	}
// }
//
// func (m *MemoryPercent) sample() {
// 	virtualMem, _ := mem.VirtualMemory()
// 	m.samples = append(m.samples, virtualMem.UsedPercent)
// }
//
// type MemoryAvailable struct {
// 	Sample
// 	name string
// }
//
// func NewMemoryAvailable() *MemoryAvailable {
// 	return &MemoryAvailable{
// 		name: "proc.memory.availableMB",
// 	}
// }
//
// func (m *MemoryAvailable) sample() {
// 	virtualMem, _ := mem.VirtualMemory()
// 	m.samples = append(m.samples, float64(virtualMem.Available)/1024/1024)
// }
//
// type Memory struct {
// 	metrics []*Sample
// 	name    string
// }
//
// func NewMemory() *Memory {
// 	return &Memory{
// 		name: "memory",
// 	}
// }
//
// func (m *Memory) start() {
// 	for _, metric := range m.metrics {
// 		metric.sample()
// 	}
// }
//
// func (m *Memory) finish() {
// 	for _, metric := range m.metrics {
// 		metric.clear()
// 	}
// }
//
// type MetricsMonitor struct {
// 	name    string
// 	metrics []Metric
// }
//
// func (m *MetricsMonitor) start() {
// 	// implementation of start goes here
// }
//
// func (m *MetricsMonitor) finish() {
// 	// implementation of finish goes here
// }
