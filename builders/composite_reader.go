package builders

import (
	"log"
	"reflect"

	"github.com/smartystreets/listeners"
	"github.com/smartystreets/messaging/v2"
	"github.com/smartystreets/pipeline/handlers"
)

type CompositeReaderBuilder struct {
	sourceQueue  string
	bindings     []string
	transformers []handlers.Transformer
	broker       messaging.MessageBroker
	types        map[string]reflect.Type
	panicMissing bool
	panicFail    bool
}

func NewCompositeReader(broker messaging.MessageBroker, sourceQueue string) *CompositeReaderBuilder {
	return &CompositeReaderBuilder{
		broker:      broker,
		sourceQueue: sourceQueue,
		types:       make(map[string]reflect.Type),
	}
}

func (this *CompositeReaderBuilder) RegisterTypes(types map[string]reflect.Type) *CompositeReaderBuilder {
	for key, value := range types {
		this.types[key] = value
	}

	return this
}

func (this *CompositeReaderBuilder) RegisterBindings(bindings []string) *CompositeReaderBuilder {
	for _, source := range bindings {
		this.bindings = append(this.bindings, source)
	}

	return this
}

func (this *CompositeReaderBuilder) PanicWhenMessageTypeNotFound() *CompositeReaderBuilder {
	this.panicMissing = true
	return this
}

func (this *CompositeReaderBuilder) PanicWhenDeserializationFails() *CompositeReaderBuilder {
	this.panicFail = true
	return this
}

func (this *CompositeReaderBuilder) AppendTransformer(value handlers.Transformer) *CompositeReaderBuilder {
	if value != nil {
		this.transformers = append(this.transformers, value)
	}

	return this
}

func (this *CompositeReaderBuilder) Build() messaging.Reader {
	receive := this.openReader()
	input := receive.Deliveries()
	output := make(chan messaging.Delivery, cap(input))

	deserializer := handlers.NewJSONDeserializer(this.types)
	if this.panicMissing {
		deserializer.PanicWhenMessageTypeIsUnknown()
	}
	if this.panicFail {
		deserializer.PanicWhenDeserializationFails()
	}

	transformers := append([]handlers.Transformer{deserializer}, this.transformers...)
	transform := handlers.NewTransformationHandler(input, output, transformers...)

	return &compositeReader{
		receive:     receive,
		deserialize: transform,
		deliveries:  output,
	}
}

func (this *CompositeReaderBuilder) openReader() messaging.Reader {
	if len(this.sourceQueue) > 0 {
		return this.broker.OpenReader(this.sourceQueue, this.bindings...)
	}

	if len(this.bindings) > 0 {
		return this.broker.OpenTransientReader(this.bindings)
	}

	log.Fatal("Unable to open reader. No source queue or bindings specified.")
	return nil
}

type compositeReader struct {
	receive     messaging.Reader
	deserialize listeners.Listener
	deliveries  chan messaging.Delivery
}

func (this *compositeReader) Listen() {
	listeners.NewCompositeWaitListener(
		this.receive,
		this.deserialize,
	).Listen()
}
func (this *compositeReader) Close() {
	this.receive.Close()
}

func (this *compositeReader) Deliveries() <-chan messaging.Delivery {
	return this.deliveries
}
func (this *compositeReader) Acknowledgements() chan<- interface{} {
	return this.receive.Acknowledgements()
}
