package handlehttp

import (
	"context"
	"encoding/json"
)

type requestContextKeyType string

const requestContextKey = requestContextKeyType("requestContextKey")

func RequestContextFromContext(ctx context.Context) *RequestContext {
	return ctx.Value(requestContextKey).(*RequestContext)
}

type RequestContext struct {
	RouteKey     string                    `json:"routeKey"`
	AccountID    string                    `json:"accountId"`
	Stage        string                    `json:"stage"`
	RequestID    string                    `json:"requestId"`
	APIID        string                    `json:"apiId"` // The API Gateway HTTP API Id
	DomainName   string                    `json:"domainName"`
	DomainPrefix string                    `json:"domainPrefix"`
	Time         string                    `json:"time"`
	TimeEpoch    int64                     `json:"timeEpoch"`
	HTTP         HTTPDescription           `json:"http"`
	Authorizer   *RequestContextAuthorizer `json:"authorizer,omitempty"`
}

type RequestContextAuthorizer struct {
	JWT    *RequestContextAuthorizerJWT `json:"jwt,omitempty"`
	Lambda json.RawMessage              `json:"lambda,omitempty"`
	IAM    *RequestContextAuthorizerIAM `json:"iam,omitempty"`
}

type RequestContextAuthorizerJWT struct {
	Claims map[string]string `json:"claims"`
	Scopes []string          `json:"scopes,omitempty"`
}

type RequestContextAuthorizerIAM struct {
	AccessKey       string                                  `json:"accessKey"`
	AccountID       string                                  `json:"accountId"`
	CallerID        string                                  `json:"callerId"`
	CognitoIdentity RequestContextAuthorizerCognitoIdentity `json:"cognitoIdentity,omitempty"`
	PrincipalOrgID  string                                  `json:"principalOrgId"`
	UserARN         string                                  `json:"userArn"`
	UserID          string                                  `json:"userId"`
}

type RequestContextAuthorizerCognitoIdentity struct {
	AMR            []string `json:"amr"`
	IdentityID     string   `json:"identityId"`
	IdentityPoolID string   `json:"identityPoolId"`
}

type HTTPDescription struct {
	Method    string `json:"method"`
	Path      string `json:"path"`
	Protocol  string `json:"protocol"`
	SourceIP  string `json:"sourceIp"`
	UserAgent string `json:"userAgent"`
}
