package authenticator

import (
	"context"
	"fmt"
	"github.com/apex/gateway"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	cip "github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
	"log"
	"net/http"
	"os"
	"github.com/gobuffalo/packr"
	"html/template"
)

var DynamoDbTableName = os.Getenv("DDB_TABLE_NAME")
var UserPoolId = os.Getenv("USER_POOL_ID")
var AppClientId = os.Getenv("APP_CLIENT_ID")
var SuccessRedirect = os.Getenv("SUCCESS_REDIRECT")
var ProtectedDomainName = os.Getenv("PROTECTED_DOMAIN_NAME")
var AuthDomainName = os.Getenv("AUTH_DOMAIN_NAME")

func AuthHandler(ddb *dynamodb.DynamoDB, cog *cip.CognitoIdentityProvider, config *oauth2.Config, t *template.Template) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. we panic/recover to simplify logic in case of error.
		defer recoverAndError(w, t)
		// 1. first we need a special cookie that is transmitted for the login and contains the unique "state"
		cookie, err := r.Cookie("monolith-state")
		if err != nil {
			panic(errors.Wrap(err, "cookie monolith-state not present"))
		}
		cState := cookie.Value
		// 2. we compare the cookie state with the state in the request.
		v := r.URL.Query()
		rState := v.Get("state")
		if cState != rState {
			panic(errors.Errorf("cState != rState; cState: %s; rState: %s", cState, rState))
		}
		// 3. we can than exchange the cookie from some access/refresh tokens. we do not need them at the time.
		//    later, we will use it to create a proper session value.
		tokens, err := config.Exchange(context.Background(), v.Get("code"))
		if err != nil {
			panic(errors.Wrap(err, "token exchange not valid"))
		}
		// 4. we fetch some user information.
		user, err := cog.GetUserRequest(&cip.GetUserInput{
			AccessToken: &tokens.AccessToken,
		}).Send()
		if err != nil {
			panic(errors.Wrap(err, "could not fetch user data"))
		}
		var email *string
		for _, attr := range user.UserAttributes {
			if *attr.Name == "email" {
				email = attr.Value
				break
			}
		}
		// 4. we can create a new session since the request is valid.
		rand, err := uuid.NewRandom()
		sessionid := rand.String()
		if err != nil {
			panic(errors.Wrap(err, "could not generate a new session id"))
		}
		// 5. we store the request on dynamodb for global access (for lambda@edge).
		_, err = ddb.PutItemRequest(&dynamodb.PutItemInput{
			TableName: &DynamoDbTableName,
			Item: map[string]dynamodb.AttributeValue{
				"sessionid": {S: &sessionid},
				"username":  {S: user.Username},
				"email":     {S: email},
			},
		}).Send()
		if err != nil {
			panic(errors.Wrap(err, "could not store the session"))
		}
		// 6. we set the monolith session cookie and redirect to the protected page.
		http.SetCookie(w, monolithStateCookie("", -1))
		http.SetCookie(w, monolithSessionCookie(sessionid, 3600))
		http.Redirect(w, r, SuccessRedirect, 302)
	}
}

func SigninHandler(config *oauth2.Config, t *template.Template) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		defer recoverAndError(w, t)
		rand, err := uuid.NewRandom()
		if err != nil {
			panic(errors.Wrap(err, "could not generate a new state"))
		}
		state := rand.String()
		http.SetCookie(w, monolithStateCookie(state, 300))
		url := config.AuthCodeURL(state)
		http.Redirect(w, r, url, 302)
	}
}

func SignoutHandler(ddb *dynamodb.DynamoDB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("monolith-session")
		if err == nil {
			sessionid := cookie.Value
			log.Printf("Signin out session %s\n", sessionid)
			if len(sessionid) > 0 {
				_, err = ddb.DeleteItemRequest(&dynamodb.DeleteItemInput{
					TableName: &DynamoDbTableName,
					Key:       map[string]dynamodb.AttributeValue{"sessionid": {S: &sessionid}},
				}).Send()
				if err != nil {
					log.Printf("Could not erase sessionid %s\n", sessionid)
				}
			}
		}
		http.SetCookie(w, monolithSessionCookie("", -1))
		http.Redirect(w, r, SuccessRedirect, 302)
	}
}

func RegisterRoutes(ddb *dynamodb.DynamoDB, cog *cip.CognitoIdentityProvider, config *oauth2.Config, t *template.Template) {
	http.HandleFunc("/auth", AuthHandler(ddb, cog, config, t))
	http.HandleFunc("/signin", SigninHandler(config, t))
	http.HandleFunc("/signout", SignoutHandler(ddb))
}

func monolithSessionCookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{Name: "monolith-session", Value: value, MaxAge: maxAge, Domain: ProtectedDomainName,
		Secure: true, HttpOnly: true, Unparsed: []string{"Same-Site", "lax"}}
}

func monolithStateCookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{Name: "monolith-state", Value: value, MaxAge: maxAge, Domain: AuthDomainName,
		Secure: true, HttpOnly: true}
}

func createConfig(cog *cip.CognitoIdentityProvider, userPoolId, clientId string) (*oauth2.Config, error) {
	pool, err := cog.DescribeUserPoolRequest(&cip.DescribeUserPoolInput{
		UserPoolId: &userPoolId,
	}).Send()
	if err != nil {
		return nil, err
	}
	client, err := cog.DescribeUserPoolClientRequest(&cip.DescribeUserPoolClientInput{
		UserPoolId: &userPoolId,
		ClientId:   &clientId,
	}).Send()
	if err != nil {
		return nil, err
	}
	return &oauth2.Config{
		ClientID:     clientId,
		ClientSecret: *client.UserPoolClient.ClientSecret,
		Scopes:       []string{"aws.cognito.signin.user.admin", "openid", "email"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  fmt.Sprintf("https://%s/login", *pool.UserPool.Domain),
			TokenURL: fmt.Sprintf("https://%s/token", *pool.UserPool.Domain),
		},
		RedirectURL: client.UserPoolClient.CallbackURLs[0],
	}, nil
}

func recoverAndError(w http.ResponseWriter, t *template.Template) {
	if err := recover(); err != nil {
		log.Printf("auth failed: %+v\n", err)
		w.WriteHeader(403)
		t.Execute(w, fmt.Sprintf("%+v", err))
	}
}

func checkEnv(name, val string ) {
	if len(val) == 0 {
		log.Fatalf("Property %s not defined.\n", name)
	} else {
		log.Printf("Propery %s is %s", name, val)
	}
}

func main() {
	checkEnv("DynamoDbTableName", DynamoDbTableName)
	checkEnv("UserPoolId", UserPoolId)
	checkEnv("AppClientId", AppClientId)
	checkEnv("SuccessRedirect", SuccessRedirect)
	checkEnv("AuthDomainName", AuthDomainName)
	checkEnv("ProtectedDomainName", ProtectedDomainName)
	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		log.Fatalf("could not get aws config: %+v\n", err)
	}
	ddb := dynamodb.New(cfg)
	cog := cip.New(cfg)
	config, err := createConfig(cog, UserPoolId, AppClientId)
	if err != nil {
		log.Fatalf("could not fetch cognito pool information: %+v\n", err)
	}
	box := packr.NewBox("resources")
	t, err := template.New("error").Parse(box.String("error.html"))
	if err != nil {
		log.Fatalf("could not init the error template: %+v\n", err)
	}
	RegisterRoutes(ddb, cog, config, t)
	log.Fatal(gateway.ListenAndServe(":3000", nil))
}
