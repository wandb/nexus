package server

import (
	"context"
	"sync"

	"github.com/wandb/wandb/nexus/pkg/service"
)

type Stream struct {
	handler   *Handler
	responder *Responder
	mailbox   *Mailbox
	settings  *Settings
	finished  bool
}

func NewStream(settings *Settings) *Stream {

	mailbox := NewMailbox()
	responder := NewResponder(mailbox)
	handler := NewHandler(responder.RespondResult, settings)
	return &Stream{responder: responder, handler: handler, mailbox: mailbox, settings: settings}
}

func (ns *Stream) Start(respondServerResponse func(context.Context, *service.ServerResponse)) {
	go ns.responder.Start(respondServerResponse)
	go ns.handler.Start()
}

func (ns *Stream) Deliver(rec *service.Record) *MailboxHandle {
	handle := ns.mailbox.Deliver(rec)
	ns.HandleRecord(rec)
	return handle
}

func (ns *Stream) HandleRecord(rec *service.Record) {
	ns.handler.HandleRecord(rec)
}

func (ns *Stream) MarkFinished() {
	ns.finished = true
}

func (ns *Stream) IsFinished() bool {
	return ns.finished
}

func (ns *Stream) GetSettings() *Settings {
	return ns.settings
}

func (ns *Stream) GetRun() *service.RunRecord {
	return ns.handler.GetRun()
}

func (ns *Stream) Close(wg *sync.WaitGroup) {
	defer wg.Done()

	if ns.IsFinished() {
		return
	}
	exitRecord := service.RunExitRecord{}
	record := service.Record{
		RecordType: &service.Record_Exit{Exit: &exitRecord},
	}
	handle := ns.Deliver(&record)
	_ = handle.wait()
	settings := ns.GetSettings()
	run := ns.GetRun()
	PrintHeadFoot(run, settings)
	ns.MarkFinished()
}
