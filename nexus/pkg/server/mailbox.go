package server

import (
	"fmt"
	"sync"

	"github.com/wandb/wandb/nexus/pkg/service"

	"strings"
)

type MailboxHandle struct {
	responseChan chan *service.Result
}

type Mailbox struct {
	handles map[string]*MailboxHandle
	mutex   sync.RWMutex
}

func NewMailbox() *Mailbox {
	mailbox := &Mailbox{handles: make(map[string]*MailboxHandle)}
	return mailbox
}

func NewMailboxHandle() *MailboxHandle {
	mbh := &MailboxHandle{responseChan: make(chan *service.Result)}
	return mbh
}

func (mbh *MailboxHandle) wait() *service.Result {
	// fmt.Println("read from chan")
	got := <-mbh.responseChan
	// fmt.Println("XXXX  read from chan, got", got)
	return got
}

func (mb *Mailbox) Deliver(rec *service.Record) *MailboxHandle {
	mb.mutex.Lock()
	defer mb.mutex.Unlock()

	uuid := "nexus:" + ShortID(12)
	rec.Control = &service.Control{MailboxSlot: uuid}
	// rec.GetControl().MailboxSlot = uuid
	// fmt.Println("mailbox record", rec)
	handle := NewMailboxHandle()
	fmt.Println("deliver uuid", uuid)
	mb.handles[uuid] = handle
	return handle
}

func (mb *Mailbox) Respond(result *service.Result) bool {
	// fmt.Println("mailbox result", result)
	// use mutex to protect handles:
	mb.mutex.RLock()
	defer mb.mutex.RUnlock()

	slot := result.GetControl().MailboxSlot
	if !strings.HasPrefix(slot, "nexus:") {
		return false
	}
	handle, ok := mb.handles[slot]
	if ok {
		handle.responseChan <- result
	}
	return ok
}
