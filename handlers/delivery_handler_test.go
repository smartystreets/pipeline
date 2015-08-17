package handlers

import (
	"time"

	"github.com/smartystreets/assertions/should"
	"github.com/smartystreets/clock"
	"github.com/smartystreets/gunit"
	"github.com/smartystreets/pipeline/messaging"
)

type DeliveryHandlerFixture struct {
	*gunit.Fixture

	now         time.Time
	input       chan messaging.Delivery
	output      chan interface{}
	writer      *FakeCommitWriter
	application *FakeApplication
	handler     *DeliveryHandler
}

func (this *DeliveryHandlerFixture) Setup() {
	this.now = clock.UTCNow()
	this.input = make(chan messaging.Delivery, 8)
	this.output = make(chan interface{}, 8)
	this.writer = &FakeCommitWriter{}
	this.application = &FakeApplication{}
	this.handler = NewDeliveryHandler(this.input, this.output, this.writer, this.application)
	clock.Freeze(this.now)
}
func (this *DeliveryHandlerFixture) Teardown() {
	clock.Restore()
}

///////////////////////////////////////////////////////////////

func (this *DeliveryHandlerFixture) TestCommitCalledAtEndOfBatch() {
	this.input <- messaging.Delivery{Message: 1, Receipt: "Delivery Receipt 1"}
	this.input <- messaging.Delivery{Message: 2, Receipt: "Delivery Receipt 2"}
	this.input <- messaging.Delivery{Message: 3, Receipt: "Delivery Receipt 3"}

	close(this.input)
	this.handler.Listen()

	this.So(this.writer.commits, should.Equal, 1)
	this.So(len(this.output), should.Equal, 1)
	this.So(<-this.output, should.Equal, "Delivery Receipt 3")
}

///////////////////////////////////////////////////////////////

func (this *DeliveryHandlerFixture) TestOutputChannelClosed() {
	close(this.input)
	this.handler.Listen()

	this.So(<-this.output, should.Equal, nil)
}

///////////////////////////////////////////////////////////////

func (this *DeliveryHandlerFixture) TestApplicationGeneratedMessagesAreWritten() {
	this.input <- messaging.Delivery{Message: 10, Receipt: "Delivery Receipt 1"}
	this.input <- messaging.Delivery{Message: 11, Receipt: "Delivery Receipt 2"}
	this.input <- messaging.Delivery{Message: 12, Receipt: "Delivery Receipt 3"}

	close(this.input)
	this.handler.Listen()

	this.So(this.writer.written, should.Resemble, []messaging.Dispatch{
		{Message: 1},
		{Message: 2},
		{Message: 3},
	})

	this.So(this.writer.commits, should.Equal, 1)
	this.So(len(this.output), should.Equal, 1)
	this.So(<-this.output, should.Equal, "Delivery Receipt 3")
}

///////////////////////////////////////////////////////////////

func (this *DeliveryHandlerFixture) TestNilMessagesAreNotWritten() {
	this.input <- messaging.Delivery{Message: "nil", Receipt: "Delivery Receipt"}

	close(this.input)
	this.handler.Listen()

	this.So(this.writer.written, should.BeEmpty)
	this.So(this.writer.commits, should.Equal, 1)
	this.So(len(this.output), should.Equal, 1)
	this.So(<-this.output, should.Equal, "Delivery Receipt")
}

///////////////////////////////////////////////////////////////

func (this *DeliveryHandlerFixture) TestMessageSlicesAreWritten() {
	this.input <- messaging.Delivery{Message: "multiple", Receipt: "Delivery Receipt"}

	close(this.input)
	this.handler.Listen()

	this.So(this.writer.written, should.Resemble, []messaging.Dispatch{
		{Message: 1},
		{Message: 2},
		{Message: 3},
	})
	this.So(this.writer.commits, should.Equal, 1)
	this.So(len(this.output), should.Equal, 1)
	this.So(<-this.output, should.Equal, "Delivery Receipt")
}

///////////////////////////////////////////////////////////////

type FakeCommitWriter struct {
	written []messaging.Dispatch
	commits int
}

func (this *FakeCommitWriter) Write(dispatch messaging.Dispatch) error {
	this.written = append(this.written, dispatch)
	return nil
}

func (this *FakeCommitWriter) Commit() error {
	this.commits++
	return nil
}

func (this *FakeCommitWriter) Close() {
	panic("should never be called")
}

///////////////////////////////////////////////////////////////

type FakeApplication struct {
	counter int
}

func (this *FakeApplication) Handle(message interface{}) interface{} {
	if message == "nil" {
		return nil
	} else if message == "multiple" {
		return []interface{}{1, 2, 3}
	}

	this.counter++
	return this.counter
}

///////////////////////////////////////////////////////////////