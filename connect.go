package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/darren0609/Contact-Reviewer/o365"

	"golang.org/x/oauth2"
)

var (
	client       *http.Client
	clientID     string
	clientSecret string
	guid         string
	user         User
	contact      ContactHeader
)

// D is Data for template
type D map[string]interface{}

// User represents a Microsoft Graph user
type User struct {
	Username  string `json:"displayName"`
	Email     string `json:"mail"`
	Surname   string `json:"surname"`
	Givenname string `json:"givenname"`
}

// ContactData is the contact detail
type ContactData struct {
	Etag                 string   `json:"odata.etag"`
	ID                   string   `json:"id"`
	Createddatetime      string   `json:"createdDateTime"`
	Lastmodifieddatetime string   `json:"lastModifiedDateTime"`
	Changekey            string   `json:"changeKey"`
	Displayname          string   `json:"displayName"`
	Officephone          []string `json:"businessPhones"`
	Homephone            []string `json:"homePhones"`
	Emailaddresses       []string `json:"odata.microsoft.graph.emailAddress"`
	Givenname            string   `json:"givenName"`
	Mobilephone          string   `json:"mobilePhone"`
	Surname              string   `json:"surname"`
}

// ContactHeader is the contact header and detail
type ContactHeader struct {
	Context  string        `json:"odata.context"`
	NextLink string        `json:"odata.nextLink"`
	Contacts []ContactData `json:"value"`
}

// Body is generally the body of the returned HTML when executing.
type Body struct {
	ContentType string
	Content     string
}

// EmailAddress is the Email Address of the user
type EmailAddress struct {
	Address string
}

// Recipient is the Receipient of the email you wish to send
type Recipient struct {
	EmailAddress EmailAddress
}

// Message is the specifics of the message being sent
type Message struct {
	Subject      string
	Body         Body
	ToRecipients []Recipient
}

func getContacts(w http.ResponseWriter, r *http.Request) {
	var fullcont ContactHeader

	// Use OData query parameters to control the results
	// - Only first 10 results returned
	// - Only return the GivenName, Surname, and EmailAddresses fields
	// - Sort the results by the GivenName field in ascending order
	//query_parameters := "$top=50",
	//	"$select" : "givenName,surname,emailAddresses",
	//	"$orderby": "givenName ASC",
	//}

	// Post the message to the graph API endpoint for sending email
	endpointURL := "https://graph.microsoft.com/v1.0/me/contacts"
	//$orderby=givenName ASC$top=50"
	//endpointURL := "https://graph.microsoft.com/v1.0/me/contacts"

	res, err := client.Get(endpointURL)
	if err != nil {
		fmt.Println("Error in get contacts:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	defer res.Body.Close()

	bytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		panic(err)
	}

	err = json.Unmarshal(bytes, &fullcont)
	if err != nil {
		log.Println("Failed to UNMARSHAL user data:", err)
	}

	// Parse template for response to app client
	t2, err := template.ParseFiles("tpl/contacts.html")
	if err != nil {
		fmt.Println("Error parsing template:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	t2.Execute(w, D{
		"me":          user,
		"contact":     fullcont,
		"showSuccess": false,
		"showError":   false,
	})
	return
}

// Handler for home page
func indexHandler(w http.ResponseWriter, r *http.Request) {
	//  Log into the Office 365 site - and authenticate
	t, err := template.ParseFiles("tpl/connect.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	t.Execute(w, struct{}{})

}

// Handler for login route
func loginHandler(w http.ResponseWriter, r *http.Request) {
	// guid should be long random string, find golang uuid lib?
	// guid used to only accept initiated logins, compared after response later
	if guid == "" {
		guid = "wbvcewiqf923h8vh139fh3491v"
	}
	conf := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       []string{"User.Read", "Contacts.Read", "Mail.Send"},
		RedirectURL:  "http://localhost:8080/login",
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
			TokenURL: "https://login.microsoftonline.com/common/oauth2/v2.0/token",
		},
	}
	var code string
	code = r.URL.Query().Get("code")
	if len(code) == 0 {
		// Redirect user to consent page to ask for permission
		// for the scopes specified above.
		authurl := conf.AuthCodeURL(guid, oauth2.AccessTypeOffline)
		http.Redirect(w, r, authurl, http.StatusSeeOther)
		// fmt.Printf(authurl)
		return
	}
	// Before calling Exchange, be sure to validate FormValue("state").
	if r.FormValue("state") != guid {
		log.Fatal("State has been messed with, end authentication")
		// reset state to prevent re-use
		guid = ""
	}
	ctx := context.Background()
	tok, err := conf.Exchange(ctx, code)
	if err != nil {
		log.Fatal(err)
	}
	client = conf.Client(ctx, tok)

	// Grab credentials so we can use them in displaying form detail going forward
	res, err := client.Get("https://graph.microsoft.com/v1.0/me")

	if err != nil {
		log.Println("Failed to get user/me:", err)
		return
	}
	defer res.Body.Close()
	err = json.NewDecoder(res.Body).Decode(&user)
	if err != nil {
		log.Println("Failed to parse user data:", err)
	}

	http.Redirect(w, r, "/home", http.StatusSeeOther)
	return
}

// Handler for logout route
func logoutHandler(w http.ResponseWriter, r *http.Request) {
	// reset client to forget token
	client = http.DefaultClient
	http.Redirect(w, r, "/", http.StatusSeeOther)
	return
}

// Handler for main route
func mainHandler(w http.ResponseWriter, r *http.Request) {
	res, err := client.Get("https://graph.microsoft.com/v1.0/me")
	if err != nil {
		log.Println("Failed to get user/me:", err)
	}
	defer res.Body.Close()
	err = json.NewDecoder(res.Body).Decode(&user)
	if err != nil {
		log.Println("Failed to parse user data:", err)
	}

	t, err := template.ParseFiles("tpl/main.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusSeeOther)
	}
	t.Execute(w, D{
		"me":          user,
		"showSuccess": false,
		"showError":   false,
	})
}

// Handler for sendmail route
func sendMailHandler(w http.ResponseWriter, r *http.Request) {
	// Create the email to be sent via the Graph API
	var emailBody bytes.Buffer
	t, err := template.ParseFiles("tpl/email.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	t.Execute(&emailBody, user.Username)
	// Gather and encode payload data for posting email message to graph
	recip := Recipient{}
	recip.EmailAddress.Address = r.URL.Query().Get("emailAddress")
	recips := []Recipient{recip}
	msg := D{
		"Message": Message{
			Subject: "Welcome to the Microsoft Graph Connect sample on GOLANG",
			Body: Body{
				ContentType: "HTML",
				Content:     emailBody.String(),
			},
			ToRecipients: recips,
		},
	}
	postJSON := new(bytes.Buffer)

	err = json.NewEncoder(postJSON).Encode(msg)
	if err != nil {
		fmt.Println("error encoding msg to json:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	// Post the message to the graph API endpoint for sending email
	endpointURL := "https://graph.microsoft.com/v1.0/me/sendMail"
	res, err := client.Post(endpointURL, "application/json", postJSON)

	if err != nil {
		fmt.Println("error posting msg:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	// Parse template for response to app client
	t2, err := template.ParseFiles("tpl/home.html")
	if err != nil {
		fmt.Println("Error parsing template:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	// Graph API will respond with 202 if sendmail request was successful
	if res.StatusCode == 202 {
		t2.Execute(w, D{
			"me":          user,
			"showSuccess": true,
			"showError":   false,
			"recipient":   recip.EmailAddress.Address,
		})
	} else {
		t2.Execute(w, D{
			"me":          user,
			"showSuccess": false,
			"showError":   true,
		})
	}
}

// Handler for sendmail route
func sendMailHandlerNew(w http.ResponseWriter, r *http.Request) {
	// Create the email to be sent via the Graph API
	var emailBody bytes.Buffer
	t, err := template.ParseFiles("tpl/email.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	t.Execute(&emailBody, user.Username)
	// Gather and encode payload data for posting email message to graph
	recip := Recipient{}
	recip.EmailAddress.Address = r.URL.Query().Get("emailAddress")
	recips := []Recipient{recip}
	msg := D{
		"Message": Message{
			Subject: "Welcome to the Microsoft Graph Connect sample for Python",
			Body: Body{
				ContentType: "HTML",
				Content:     emailBody.String(),
			},
			ToRecipients: recips,
		},
	}
	postJSON := new(bytes.Buffer)
	err = json.NewEncoder(postJSON).Encode(msg)
	if err != nil {
		fmt.Println("error encoding msg to json:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	// Post the message to the graph API endpoint for sending email
	endpointURL := "https://graph.microsoft.com/v1.0/me/sendMail"
	res, err := client.Post(endpointURL, "application/json", postJSON)
	if err != nil {
		fmt.Println("error posting msg:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	// Parse template for response to app client
	t2, err := template.ParseFiles("tpl/home.html")
	if err != nil {
		fmt.Println("Error parsing template:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	// Graph API will respond with 202 if sendmail request was successful
	if res.StatusCode == 202 {
		t2.Execute(w, D{
			"me":          user,
			"showSuccess": true,
			"showError":   false,
			"recipient":   recip.EmailAddress.Address,
		})
	} else {
		t2.Execute(w, D{
			"me":          user,
			"showSuccess": false,
			"showError":   true,
		})
	}
}

func main() {
	var err error
	configFile := "private.txt"
	//templateFolder := "templates/"
	clientID, clientSecret, err = o365.GetCreds(configFile)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	http.HandleFunc("/", loginHandler)
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/logout", logoutHandler)
	http.HandleFunc("/main", mainHandler)
	http.HandleFunc("/contacts", getContacts)
	http.HandleFunc("/home", sendMailHandlerNew)
	http.HandleFunc("/sendmail", sendMailHandler)
	http.ListenAndServe(":8080", nil)
	// fmt.Println("Success", client.Head)
}
