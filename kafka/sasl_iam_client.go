package kafka

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	sign "github.com/aws/aws-sdk-go/aws/signer/v4"
)

type State int

const (
	SEND_CLIENT_FIRST_MESSAGE State = iota
	RECEIVE_SERVER_RESPONSE
	COMPLETE
	FAILED
)

const SASLTypeAWSIAM string = "AWS_MSK_IAM"

type IAMSaslClient struct {
	brokerHost string 
	state      State
	region     string
	userAgent  string
}

type AuthResponse struct {
	Version   string `json:"version"`
	RequestId string `json:"request-id"`
}

func (x *IAMSaslClient) CreateAuthPayload(creds *credentials.Credentials) ([]byte, error) {
	// about sigv4 signing: https://docs.aws.amazon.com/general/latest/gr/sigv4_signing.html
	// python example: https://docs.aws.amazon.com/general/latest/gr/sigv4-signed-request-examples.html

	service := "kafka-cluster"

	// prepare a standard request for presign (using query string)
	v := url.Values{
		"action": {service + ":Connect"},
	}
	u := url.URL{
		Host: "kakfa://" + x.brokerHost,
		Path: "/",
		RawQuery: v.Encode(),
	}
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("GenerateClientMessage: failed preparing HTTP request to be signed: %w", err)
	}
	
	// sign request using query string
	signer := sign.NewSigner(creds)
	headers, err := signer.Presign(req, nil, service, x.region, 15 * time.Minute, time.Now().UTC())
	if err != nil {
		return nil, fmt.Errorf("GenerateClientMessage: failed presigning HTTP request: %w", err)
	}

	// prepare payload without x-amz headers
	payload := map[string]string{
		"version": "2020_10_22",
		"host": x.brokerHost,
		"user-agent": x.userAgent,
		"action": service + ":Connect",
	}

	// add x-amz headers in the payload
	for k, v := range headers {
		payload[strings.ToLower(k)] = v[0]
	}

	return json.Marshal(payload)
}

func (x *IAMSaslClient) GenerateClientMessage() (response string, err error) {
	creds := credentials.NewChainCredentials(
		[]credentials.Provider{
			&credentials.EnvProvider{},
			&ec2rolecreds.EC2RoleProvider{Client: ec2metadata.New(session.New(&aws.Config{Region: &x.region}))},
		})
	
	if err != nil {
		return "", fmt.Errorf("GenerateClientMessage: failed getting AWS credentials: %w", err)
	}

	payload, err := x.CreateAuthPayload(creds)
	if err != nil {
		return "", fmt.Errorf("GenerateClientMessage: failed creating MSK IAM payload: %w", err)
	}

	return string(payload), nil
}

func (x *IAMSaslClient) Begin(userName, password, authzID string) (err error) {
	if x.brokerHost == "" {
		return errors.New("Begin: The MSK broker hostname could not be an empty value.")
	}

	if x.region == "" {
		return errors.New("Begin: The AWS region could not be an empty value.")
	}

	if x.userAgent == "" {
		x.userAgent = "sasl-iam-client-terraform-kafka-provider"
	}
	x.state = SEND_CLIENT_FIRST_MESSAGE
	return nil
}

func (x *IAMSaslClient) Step(challenge string) (response string, err error) {
	switch x.state {
	case SEND_CLIENT_FIRST_MESSAGE:
		// generate client message
		if challenge != "" {
			// we expect empty challenge at the beginning of the auth flow
			return "", fmt.Errorf("Step expects an empty challenge in state 'SEND_CLIENT_FIRST_MESSAGE'")
		}
		response, err = x.GenerateClientMessage()
		if err != nil {
			x.state = FAILED
			return "", err
		}
		x.state = RECEIVE_SERVER_RESPONSE
	case RECEIVE_SERVER_RESPONSE:
		// we expect non-empty challenge
		if challenge == "" {
			// we expect non-empty challenge from server response
			x.state = FAILED
			return "", fmt.Errorf("Step expects a non-empty authentication response in state 'RECEIVE_SERVER_RESPONSE'")
		}

		// decode response
		var authResponse AuthResponse
		if err := json.NewDecoder(strings.NewReader(challenge)).Decode(&authResponse); err != nil {
			x.state = FAILED
			return "", fmt.Errorf("Response from broker server is not a valid JSON.")
		}
		log.Printf("[DEBUG] SASL IAM auth response from server: Version=%s, RequestId=%s", authResponse.Version, authResponse.RequestId)
		x.state = COMPLETE
	default:
		return "", errors.New("Challenge received in unexpected state")
	}
	
	return response, nil
}

func (x *IAMSaslClient) Done() bool {
	return x.state == COMPLETE
}
