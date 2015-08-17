package persist

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/smartystreets/assertions/should"
	"github.com/smartystreets/gunit"
)

var (
	retries     int = 5
	maxAttempts int = retries + 1
)

type PutRetryClientFixture struct {
	*gunit.Fixture

	fakeClient  *FakeHTTPClientForPutRetry
	retryClient *PutRetryClient
	response    *http.Response
	err         error
	naps        int
	stdout      *bytes.Buffer
}

func (this *PutRetryClientFixture) Setup() {
	this.stdout = new(bytes.Buffer)
	log.SetOutput(this.stdout)
	napTime = func(time.Duration) { this.naps++ }
	this.fakeClient = &FakeHTTPClientForPutRetry{}
	this.retryClient = NewPutRetryClient(this.fakeClient, retries)
}

////////////////////////////////////////////////////////////////////

func (this *PutRetryClientFixture) TestClientSucceedsOnFirstTry() {
	request := buildRequestFromPath("/")
	this.response, this.err = this.retryClient.Do(request)
	this.assertResponseAndNoError()
}

////////////////////////////////////////////////////////////////////

func (this *PutRetryClientFixture) TestClientFailsAtFirst_ThenSucceeds() {
	request := buildRequestFromPath("/fail-first")

	this.response, this.err = this.retryClient.Do(request)

	this.assertResponseAndNoError()
	this.assertPayloadIsIdenticalOnEveryRequest()
	this.assertAllAttemptsUsed()
}

////////////////////////////////////////////////////////////////////

func (this *PutRetryClientFixture) TestClientNeverSucceeds() {
	request := buildRequestFromPath("/fail-always")

	this.response, this.err = this.retryClient.Do(request)

	this.assertNoResponseAndError()
	this.assertPayloadIsIdenticalOnEveryRequest()
	this.assertAllAttemptsUsed()
	this.assertWaitingPeriodBetweenAttempts()
}

////////////////////////////////////////////////////////////////////

func (this *PutRetryClientFixture) TestClientRetriesBadStatus_ThenSucceeds() {
	request := buildRequestFromPath("/bad-status")

	this.response, this.err = this.retryClient.Do(request)

	this.assertResponseAndNoError()
	this.assertPayloadIsIdenticalOnEveryRequest()
	this.assertAllAttemptsUsed()
	this.assertBodySentToStdOut()
}

func (this *PutRetryClientFixture) assertBodySentToStdOut() {
	this.So(this.stdout.String(), should.ContainSubstring, "Not Found")
}

////////////////////////////////////////////////////////////////////

func buildRequestFromPath(path string) *http.Request {
	request, _ := http.NewRequest("GET", path, nil)
	request.Body = newNopCloser([]byte(bodyPayload))
	return request
}
func (this *PutRetryClientFixture) assertResponseAndNoError() {
	this.So(this.response, should.NotBeNil)
	this.So(this.err, should.BeNil)
}
func (this *PutRetryClientFixture) assertNoResponseAndError() {
	this.So(this.response, should.BeNil)
	this.So(this.err, should.NotBeNil)
}
func (this *PutRetryClientFixture) assertAllAttemptsUsed() {
	this.So(this.fakeClient.calls, should.Equal, maxAttempts)
}
func (this *PutRetryClientFixture) assertWaitingPeriodBetweenAttempts() {
	this.So(this.naps, should.Equal, maxAttempts)
}
func (this *PutRetryClientFixture) assertPayloadIsIdenticalOnEveryRequest() {
	if len(this.fakeClient.bodies) == 0 {
		return
	}

	for _, item := range this.fakeClient.bodies {
		this.So(string(item), should.Equal, bodyPayload)
	}
}

const bodyPayload = "Hello, World!"

////////////////////////////////////////////////////////////////////

type FakeHTTPClientForPutRetry struct {
	calls  int
	bodies [][]byte
}

func (this *FakeHTTPClientForPutRetry) Do(request *http.Request) (*http.Response, error) {
	body, _ := ioutil.ReadAll(request.Body)
	this.bodies = append(this.bodies, body)
	request.Body.Close()

	this.calls++
	if request.URL.Path == "/fail-first" && this.calls < maxAttempts {
		return nil, errors.New("GOPHERS!")
	} else if request.URL.Path == "/fail-always" {
		return nil, errors.New("GOPHERS!")
	} else if request.URL.Path == "/bad-status" && this.calls < maxAttempts {
		return PutRetry_NotFoundResponse, nil
	} else {
		return &http.Response{StatusCode: 200}, nil
	}
}

var (
	PutRetry_NotFoundResponse = &http.Response{StatusCode: 404, Body: newFakeBody("Not Found")}
)

//////////////////////////////////////////////////////

func newFakeBody(message string) io.ReadCloser {
	return &ClosingReader{Reader: strings.NewReader(message)}
}

type ClosingReader struct {
	*strings.Reader
	closed bool
}

func (this *ClosingReader) Close() error {
	this.closed = true
	return nil
}