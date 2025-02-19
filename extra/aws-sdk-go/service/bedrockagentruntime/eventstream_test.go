// Code generated by private/model/cli/gen-api/main.go. DO NOT EDIT.

//go:build go1.16
// +build go1.16

package bedrockagentruntime

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/corehandlers"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/awstesting/unit"
	"github.com/aws/aws-sdk-go/private/protocol"
	"github.com/aws/aws-sdk-go/private/protocol/eventstream"
	"github.com/aws/aws-sdk-go/private/protocol/eventstream/eventstreamapi"
	"github.com/aws/aws-sdk-go/private/protocol/eventstream/eventstreamtest"
	"github.com/aws/aws-sdk-go/private/protocol/restjson"
)

var _ time.Time
var _ awserr.Error
var _ context.Context
var _ sync.WaitGroup
var _ strings.Reader

func TestInvokeAgent_Read(t *testing.T) {
	expectEvents, eventMsgs := mockInvokeAgentReadEvents()
	sess, cleanupFn, err := eventstreamtest.SetupEventStreamSession(t,
		eventstreamtest.ServeEventStream{
			T:      t,
			Events: eventMsgs,
		},
		true,
	)
	if err != nil {
		t.Fatalf("expect no error, %v", err)
	}
	defer cleanupFn()

	svc := New(sess)
	resp, err := svc.InvokeAgent(nil)
	if err != nil {
		t.Fatalf("expect no error got, %v", err)
	}
	defer resp.GetStream().Close()

	var i int
	for event := range resp.GetStream().Events() {
		if event == nil {
			t.Errorf("%d, expect event, got nil", i)
		}
		if e, a := expectEvents[i], event; !reflect.DeepEqual(e, a) {
			t.Errorf("%d, expect %T %v, got %T %v", i, e, e, a, a)
		}
		i++
	}

	if err := resp.GetStream().Err(); err != nil {
		t.Errorf("expect no error, %v", err)
	}
}

func TestInvokeAgent_ReadClose(t *testing.T) {
	_, eventMsgs := mockInvokeAgentReadEvents()
	sess, cleanupFn, err := eventstreamtest.SetupEventStreamSession(t,
		eventstreamtest.ServeEventStream{
			T:      t,
			Events: eventMsgs,
		},
		true,
	)
	if err != nil {
		t.Fatalf("expect no error, %v", err)
	}
	defer cleanupFn()

	svc := New(sess)
	resp, err := svc.InvokeAgent(nil)
	if err != nil {
		t.Fatalf("expect no error got, %v", err)
	}

	// Assert calling Err before close does not close the stream.
	resp.GetStream().Err()
	select {
	case _, ok := <-resp.GetStream().Events():
		if !ok {
			t.Fatalf("expect stream not to be closed, but was")
		}
	default:
	}

	resp.GetStream().Close()
	<-resp.GetStream().Events()

	if err := resp.GetStream().Err(); err != nil {
		t.Errorf("expect no error, %v", err)
	}
}

func TestInvokeAgent_ReadUnknownEvent(t *testing.T) {
	expectEvents, eventMsgs := mockInvokeAgentReadEvents()
	var eventOffset int

	unknownEvent := eventstream.Message{
		Headers: eventstream.Headers{
			eventstreamtest.EventMessageTypeHeader,
			{
				Name:  eventstreamapi.EventTypeHeader,
				Value: eventstream.StringValue("UnknownEventName"),
			},
		},
		Payload: []byte("some unknown event"),
	}

	eventMsgs = append(eventMsgs[:eventOffset],
		append([]eventstream.Message{unknownEvent}, eventMsgs[eventOffset:]...)...)

	expectEvents = append(expectEvents[:eventOffset],
		append([]ResponseStreamEvent{
			&ResponseStreamUnknownEvent{
				Type:    "UnknownEventName",
				Message: unknownEvent,
			},
		},
			expectEvents[eventOffset:]...)...)

	sess, cleanupFn, err := eventstreamtest.SetupEventStreamSession(t,
		eventstreamtest.ServeEventStream{
			T:      t,
			Events: eventMsgs,
		},
		true,
	)
	if err != nil {
		t.Fatalf("expect no error, %v", err)
	}
	defer cleanupFn()

	svc := New(sess)
	resp, err := svc.InvokeAgent(nil)
	if err != nil {
		t.Fatalf("expect no error got, %v", err)
	}
	defer resp.GetStream().Close()

	var i int
	for event := range resp.GetStream().Events() {
		if event == nil {
			t.Errorf("%d, expect event, got nil", i)
		}
		if e, a := expectEvents[i], event; !reflect.DeepEqual(e, a) {
			t.Errorf("%d, expect %T %v, got %T %v", i, e, e, a, a)
		}
		i++
	}

	if err := resp.GetStream().Err(); err != nil {
		t.Errorf("expect no error, %v", err)
	}
}

func BenchmarkInvokeAgent_Read(b *testing.B) {
	_, eventMsgs := mockInvokeAgentReadEvents()
	var buf bytes.Buffer
	encoder := eventstream.NewEncoder(&buf)
	for _, msg := range eventMsgs {
		if err := encoder.Encode(msg); err != nil {
			b.Fatalf("failed to encode message, %v", err)
		}
	}
	stream := &loopReader{source: bytes.NewReader(buf.Bytes())}

	sess := unit.Session
	svc := New(sess, &aws.Config{
		Endpoint:               aws.String("https://example.com"),
		DisableParamValidation: aws.Bool(true),
	})
	svc.Handlers.Send.Swap(corehandlers.SendHandler.Name,
		request.NamedHandler{Name: "mockSend",
			Fn: func(r *request.Request) {
				r.HTTPResponse = &http.Response{
					Status:     "200 OK",
					StatusCode: 200,
					Header:     http.Header{},
					Body:       ioutil.NopCloser(stream),
				}
			},
		},
	)

	resp, err := svc.InvokeAgent(nil)
	if err != nil {
		b.Fatalf("failed to create request, %v", err)
	}
	defer resp.GetStream().Close()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if err = resp.GetStream().Err(); err != nil {
			b.Fatalf("expect no error, got %v", err)
		}
		event := <-resp.GetStream().Events()
		if event == nil {
			b.Fatalf("expect event, got nil, %v, %d", resp.GetStream().Err(), i)
		}
	}
}

func mockInvokeAgentReadEvents() (
	[]ResponseStreamEvent,
	[]eventstream.Message,
) {
	expectEvents := []ResponseStreamEvent{
		&PayloadPart{
			Attribution: &Attribution{
				Citations: []*Citation{
					{
						GeneratedResponsePart: &GeneratedResponsePart{
							TextResponsePart: &TextResponsePart{
								Span: &Span{
									End:   aws.Int64(123),
									Start: aws.Int64(123),
								},
								Text: aws.String("string value goes here"),
							},
						},
						RetrievedReferences: []*RetrievedReference{
							{
								Content: &RetrievalResultContent{
									Text: aws.String("string value goes here"),
								},
								Location: &RetrievalResultLocation{
									S3Location: &RetrievalResultS3Location{
										Uri: aws.String("string value goes here"),
									},
									Type: aws.String("string value goes here"),
								},
							},
							{
								Content: &RetrievalResultContent{
									Text: aws.String("string value goes here"),
								},
								Location: &RetrievalResultLocation{
									S3Location: &RetrievalResultS3Location{
										Uri: aws.String("string value goes here"),
									},
									Type: aws.String("string value goes here"),
								},
							},
							{
								Content: &RetrievalResultContent{
									Text: aws.String("string value goes here"),
								},
								Location: &RetrievalResultLocation{
									S3Location: &RetrievalResultS3Location{
										Uri: aws.String("string value goes here"),
									},
									Type: aws.String("string value goes here"),
								},
							},
						},
					},
					{
						GeneratedResponsePart: &GeneratedResponsePart{
							TextResponsePart: &TextResponsePart{
								Span: &Span{
									End:   aws.Int64(123),
									Start: aws.Int64(123),
								},
								Text: aws.String("string value goes here"),
							},
						},
						RetrievedReferences: []*RetrievedReference{
							{
								Content: &RetrievalResultContent{
									Text: aws.String("string value goes here"),
								},
								Location: &RetrievalResultLocation{
									S3Location: &RetrievalResultS3Location{
										Uri: aws.String("string value goes here"),
									},
									Type: aws.String("string value goes here"),
								},
							},
							{
								Content: &RetrievalResultContent{
									Text: aws.String("string value goes here"),
								},
								Location: &RetrievalResultLocation{
									S3Location: &RetrievalResultS3Location{
										Uri: aws.String("string value goes here"),
									},
									Type: aws.String("string value goes here"),
								},
							},
							{
								Content: &RetrievalResultContent{
									Text: aws.String("string value goes here"),
								},
								Location: &RetrievalResultLocation{
									S3Location: &RetrievalResultS3Location{
										Uri: aws.String("string value goes here"),
									},
									Type: aws.String("string value goes here"),
								},
							},
						},
					},
					{
						GeneratedResponsePart: &GeneratedResponsePart{
							TextResponsePart: &TextResponsePart{
								Span: &Span{
									End:   aws.Int64(123),
									Start: aws.Int64(123),
								},
								Text: aws.String("string value goes here"),
							},
						},
						RetrievedReferences: []*RetrievedReference{
							{
								Content: &RetrievalResultContent{
									Text: aws.String("string value goes here"),
								},
								Location: &RetrievalResultLocation{
									S3Location: &RetrievalResultS3Location{
										Uri: aws.String("string value goes here"),
									},
									Type: aws.String("string value goes here"),
								},
							},
							{
								Content: &RetrievalResultContent{
									Text: aws.String("string value goes here"),
								},
								Location: &RetrievalResultLocation{
									S3Location: &RetrievalResultS3Location{
										Uri: aws.String("string value goes here"),
									},
									Type: aws.String("string value goes here"),
								},
							},
							{
								Content: &RetrievalResultContent{
									Text: aws.String("string value goes here"),
								},
								Location: &RetrievalResultLocation{
									S3Location: &RetrievalResultS3Location{
										Uri: aws.String("string value goes here"),
									},
									Type: aws.String("string value goes here"),
								},
							},
						},
					},
				},
			},
			Bytes: []byte("blob value goes here"),
		},
		&TracePart{
			AgentAliasId: aws.String("string value goes here"),
			AgentId:      aws.String("string value goes here"),
			SessionId:    aws.String("string value goes here"),
			Trace: &Trace{
				FailureTrace: &FailureTrace{
					FailureReason: aws.String("string value goes here"),
					TraceId:       aws.String("string value goes here"),
				},
				OrchestrationTrace: &OrchestrationTrace{
					InvocationInput: &InvocationInput_{
						ActionGroupInvocationInput: &ActionGroupInvocationInput_{
							ActionGroupName: aws.String("string value goes here"),
							ApiPath:         aws.String("string value goes here"),
							Parameters: []*Parameter{
								{
									Name:  aws.String("string value goes here"),
									Type:  aws.String("string value goes here"),
									Value: aws.String("string value goes here"),
								},
								{
									Name:  aws.String("string value goes here"),
									Type:  aws.String("string value goes here"),
									Value: aws.String("string value goes here"),
								},
								{
									Name:  aws.String("string value goes here"),
									Type:  aws.String("string value goes here"),
									Value: aws.String("string value goes here"),
								},
							},
							RequestBody: &RequestBody{
								Content: map[string][]*Parameter{
									"a": {
										{
											Name:  aws.String("string value goes here"),
											Type:  aws.String("string value goes here"),
											Value: aws.String("string value goes here"),
										},
										{
											Name:  aws.String("string value goes here"),
											Type:  aws.String("string value goes here"),
											Value: aws.String("string value goes here"),
										},
										{
											Name:  aws.String("string value goes here"),
											Type:  aws.String("string value goes here"),
											Value: aws.String("string value goes here"),
										},
									},
									"b": {
										{
											Name:  aws.String("string value goes here"),
											Type:  aws.String("string value goes here"),
											Value: aws.String("string value goes here"),
										},
										{
											Name:  aws.String("string value goes here"),
											Type:  aws.String("string value goes here"),
											Value: aws.String("string value goes here"),
										},
										{
											Name:  aws.String("string value goes here"),
											Type:  aws.String("string value goes here"),
											Value: aws.String("string value goes here"),
										},
									},
									"c": {
										{
											Name:  aws.String("string value goes here"),
											Type:  aws.String("string value goes here"),
											Value: aws.String("string value goes here"),
										},
										{
											Name:  aws.String("string value goes here"),
											Type:  aws.String("string value goes here"),
											Value: aws.String("string value goes here"),
										},
										{
											Name:  aws.String("string value goes here"),
											Type:  aws.String("string value goes here"),
											Value: aws.String("string value goes here"),
										},
									},
								},
							},
							Verb: aws.String("string value goes here"),
						},
						InvocationType: aws.String("string value goes here"),
						KnowledgeBaseLookupInput: &KnowledgeBaseLookupInput_{
							KnowledgeBaseId: aws.String("string value goes here"),
							Text:            aws.String("string value goes here"),
						},
						TraceId: aws.String("string value goes here"),
					},
					ModelInvocationInput: &ModelInvocationInput_{
						InferenceConfiguration: &InferenceConfiguration{
							MaximumLength: aws.Int64(123),
							StopSequences: []*string{
								aws.String("string value goes here"),
								aws.String("string value goes here"),
								aws.String("string value goes here"),
							},
							Temperature: aws.Float64(123.4),
							TopK:        aws.Int64(123),
							TopP:        aws.Float64(123.4),
						},
						OverrideLambda:     aws.String("string value goes here"),
						ParserMode:         aws.String("string value goes here"),
						PromptCreationMode: aws.String("string value goes here"),
						Text:               aws.String("string value goes here"),
						TraceId:            aws.String("string value goes here"),
						Type:               aws.String("string value goes here"),
					},
					Observation: &Observation{
						ActionGroupInvocationOutput: &ActionGroupInvocationOutput_{
							Text: aws.String("string value goes here"),
						},
						FinalResponse: &FinalResponse{
							Text: aws.String("string value goes here"),
						},
						KnowledgeBaseLookupOutput: &KnowledgeBaseLookupOutput_{
							RetrievedReferences: []*RetrievedReference{
								{
									Content: &RetrievalResultContent{
										Text: aws.String("string value goes here"),
									},
									Location: &RetrievalResultLocation{
										S3Location: &RetrievalResultS3Location{
											Uri: aws.String("string value goes here"),
										},
										Type: aws.String("string value goes here"),
									},
								},
								{
									Content: &RetrievalResultContent{
										Text: aws.String("string value goes here"),
									},
									Location: &RetrievalResultLocation{
										S3Location: &RetrievalResultS3Location{
											Uri: aws.String("string value goes here"),
										},
										Type: aws.String("string value goes here"),
									},
								},
								{
									Content: &RetrievalResultContent{
										Text: aws.String("string value goes here"),
									},
									Location: &RetrievalResultLocation{
										S3Location: &RetrievalResultS3Location{
											Uri: aws.String("string value goes here"),
										},
										Type: aws.String("string value goes here"),
									},
								},
							},
						},
						RepromptResponse: &RepromptResponse{
							Source: aws.String("string value goes here"),
							Text:   aws.String("string value goes here"),
						},
						TraceId: aws.String("string value goes here"),
						Type:    aws.String("string value goes here"),
					},
					Rationale: &Rationale{
						Text:    aws.String("string value goes here"),
						TraceId: aws.String("string value goes here"),
					},
				},
				PostProcessingTrace: &PostProcessingTrace{
					ModelInvocationInput: &ModelInvocationInput_{
						InferenceConfiguration: &InferenceConfiguration{
							MaximumLength: aws.Int64(123),
							StopSequences: []*string{
								aws.String("string value goes here"),
								aws.String("string value goes here"),
								aws.String("string value goes here"),
							},
							Temperature: aws.Float64(123.4),
							TopK:        aws.Int64(123),
							TopP:        aws.Float64(123.4),
						},
						OverrideLambda:     aws.String("string value goes here"),
						ParserMode:         aws.String("string value goes here"),
						PromptCreationMode: aws.String("string value goes here"),
						Text:               aws.String("string value goes here"),
						TraceId:            aws.String("string value goes here"),
						Type:               aws.String("string value goes here"),
					},
					ModelInvocationOutput: &PostProcessingModelInvocationOutput_{
						ParsedResponse: &PostProcessingParsedResponse{
							Text: aws.String("string value goes here"),
						},
						TraceId: aws.String("string value goes here"),
					},
				},
				PreProcessingTrace: &PreProcessingTrace{
					ModelInvocationInput: &ModelInvocationInput_{
						InferenceConfiguration: &InferenceConfiguration{
							MaximumLength: aws.Int64(123),
							StopSequences: []*string{
								aws.String("string value goes here"),
								aws.String("string value goes here"),
								aws.String("string value goes here"),
							},
							Temperature: aws.Float64(123.4),
							TopK:        aws.Int64(123),
							TopP:        aws.Float64(123.4),
						},
						OverrideLambda:     aws.String("string value goes here"),
						ParserMode:         aws.String("string value goes here"),
						PromptCreationMode: aws.String("string value goes here"),
						Text:               aws.String("string value goes here"),
						TraceId:            aws.String("string value goes here"),
						Type:               aws.String("string value goes here"),
					},
					ModelInvocationOutput: &PreProcessingModelInvocationOutput_{
						ParsedResponse: &PreProcessingParsedResponse{
							IsValid:   aws.Bool(true),
							Rationale: aws.String("string value goes here"),
						},
						TraceId: aws.String("string value goes here"),
					},
				},
			},
		},
	}

	var marshalers request.HandlerList
	marshalers.PushBackNamed(restjson.BuildHandler)
	payloadMarshaler := protocol.HandlerPayloadMarshal{
		Marshalers: marshalers,
	}
	_ = payloadMarshaler

	eventMsgs := []eventstream.Message{
		{
			Headers: eventstream.Headers{
				eventstreamtest.EventMessageTypeHeader,
				{
					Name:  eventstreamapi.EventTypeHeader,
					Value: eventstream.StringValue("chunk"),
				},
			},
			Payload: eventstreamtest.MarshalEventPayload(payloadMarshaler, expectEvents[0]),
		},
		{
			Headers: eventstream.Headers{
				eventstreamtest.EventMessageTypeHeader,
				{
					Name:  eventstreamapi.EventTypeHeader,
					Value: eventstream.StringValue("trace"),
				},
			},
			Payload: eventstreamtest.MarshalEventPayload(payloadMarshaler, expectEvents[1]),
		},
	}

	return expectEvents, eventMsgs
}
func TestInvokeAgent_ReadException(t *testing.T) {
	expectEvents := []ResponseStreamEvent{
		&AccessDeniedException{
			RespMetadata: protocol.ResponseMetadata{
				StatusCode: 200,
			},
			Message_: aws.String("string value goes here"),
		},
	}

	var marshalers request.HandlerList
	marshalers.PushBackNamed(restjson.BuildHandler)
	payloadMarshaler := protocol.HandlerPayloadMarshal{
		Marshalers: marshalers,
	}

	eventMsgs := []eventstream.Message{
		{
			Headers: eventstream.Headers{
				eventstreamtest.EventExceptionTypeHeader,
				{
					Name:  eventstreamapi.ExceptionTypeHeader,
					Value: eventstream.StringValue("accessDeniedException"),
				},
			},
			Payload: eventstreamtest.MarshalEventPayload(payloadMarshaler, expectEvents[0]),
		},
	}

	sess, cleanupFn, err := eventstreamtest.SetupEventStreamSession(t,
		eventstreamtest.ServeEventStream{
			T:      t,
			Events: eventMsgs,
		},
		true,
	)
	if err != nil {
		t.Fatalf("expect no error, %v", err)
	}
	defer cleanupFn()

	svc := New(sess)
	resp, err := svc.InvokeAgent(nil)
	if err != nil {
		t.Fatalf("expect no error got, %v", err)
	}

	defer resp.GetStream().Close()

	<-resp.GetStream().Events()

	err = resp.GetStream().Err()
	if err == nil {
		t.Fatalf("expect err, got none")
	}

	expectErr := &AccessDeniedException{
		RespMetadata: protocol.ResponseMetadata{
			StatusCode: 200,
		},
		Message_: aws.String("string value goes here"),
	}
	aerr, ok := err.(awserr.Error)
	if !ok {
		t.Errorf("expect exception, got %T, %#v", err, err)
	}
	if e, a := expectErr.Code(), aerr.Code(); e != a {
		t.Errorf("expect %v, got %v", e, a)
	}
	if e, a := expectErr.Message(), aerr.Message(); e != a {
		t.Errorf("expect %v, got %v", e, a)
	}

	if e, a := expectErr, aerr; !reflect.DeepEqual(e, a) {
		t.Errorf("expect error %+#v, got %+#v", e, a)
	}
}

var _ awserr.Error = (*AccessDeniedException)(nil)
var _ awserr.Error = (*BadGatewayException)(nil)
var _ awserr.Error = (*ConflictException)(nil)
var _ awserr.Error = (*DependencyFailedException)(nil)
var _ awserr.Error = (*InternalServerException)(nil)
var _ awserr.Error = (*ResourceNotFoundException)(nil)
var _ awserr.Error = (*ServiceQuotaExceededException)(nil)
var _ awserr.Error = (*ThrottlingException)(nil)
var _ awserr.Error = (*ValidationException)(nil)

type loopReader struct {
	source *bytes.Reader
}

func (c *loopReader) Read(p []byte) (int, error) {
	if c.source.Len() == 0 {
		c.source.Seek(0, 0)
	}

	return c.source.Read(p)
}
