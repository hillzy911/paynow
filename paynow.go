package main

import (
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	URL_INITIATE_TRANSACTION        = "https://www.paynow.co.zw/interface/initiatetransaction"
	URL_INITIATE_MOBILE_TRANSACTION = "https://www.paynow.co.zw/interface/remotetransaction"
)

// Here, you can define your package's functions, structs, etc.
type Paynow struct {
	ResultURL      string
	ReturnURL      string
	IntegrationID  string
	IntegrationKey string
}

// Payment represents a transaction to be sent to Paynow.
type Payment struct {
	Reference string           // Unique identifier for the transaction
	Items     [][2]interface{} // Array of items in the 'cart', each item is a tuple of title (string) and amount (float64)
	AuthEmail string           // The user's email address.
}

// PaymentResponse represents the response from Paynow during transaction initiation.
// PaymentResponse struct can handle both success and error responses.
type PaymentResponse struct {
	Status               string `json:"status"` // Common for both success and error
	BrowserURL           string `json:"browserurl,omitempty"`
	PollURL              string `json:"pollurl,omitempty"`
	Hash                 string `json:"hash,omitempty"`
	Error                string `json:"error,omitempty"` // Pointer to make it optional
	AuthorizationCode    string `json:"authorizationcode,omitempty"`
	AuthorizationExpires string `json:"authorizationexpires,omitempty"`
}

// PaymentStatusResponse defines the structure for the Paynow status update.
type PaymentStatusResponse struct {
	Reference       string
	Amount          float64
	PaynowReference string
	PollURL         string
	Status          string
	Hash            string
}

// Initialize Paynow SDK
func InitializeSDK(integrationID, integrationKey, resultURL, returnURL string) *Paynow {
	return &Paynow{
		IntegrationID:  integrationID,
		IntegrationKey: integrationKey,
		ResultURL:      resultURL,
		ReturnURL:      returnURL,
	}
}

// Create New Payment
func (p *Paynow) CreatePayment(reference, authEmail string) *Payment {
	payment := NewPayment(reference, authEmail)
	return payment
}

// Assuming the Paynow and Payment structs are defined elsewhere
func (pn *Paynow) Send(p *Payment) *PaymentResponse {

	// Generate the hash
	hash := GenerateHash(pn.ResultURL, pn.ReturnURL, p.Reference, fmt.Sprintf("%f", p.Total()), pn.IntegrationID, url.QueryEscape(p.Info()), url.QueryEscape(p.AuthEmail), "Message", pn.IntegrationKey)

	amount := math.Trunc(p.Total()*100) / 100

	uri := "https://www.paynow.co.zw/interface/initiatetransaction"
	method := "POST"

	payload := &bytes.Buffer{}
	writer := multipart.NewWriter(payload)

	// Add fields to the writer. Replace hardcoded values with values from the Payment object as needed
	_ = writer.WriteField("resulturl", pn.ResultURL)
	_ = writer.WriteField("returnurl", pn.ReturnURL)
	_ = writer.WriteField("reference", p.Reference)
	_ = writer.WriteField("amount", fmt.Sprintf("%f", amount))
	_ = writer.WriteField("id", pn.IntegrationID)
	_ = writer.WriteField("additionalinfo", url.QueryEscape(p.Info())) // URL encoded
	_ = writer.WriteField("authemail", url.QueryEscape(p.AuthEmail))   // URL encoded
	_ = writer.WriteField("status", "Message")
	_ = writer.WriteField("hash", hash)

	err := writer.Close()
	if err != nil {
		fmt.Println(err)
		return &PaymentResponse{Error: "Error closing writer"}
	}

	client := &http.Client{}
	req, err := http.NewRequest(method, uri, payload)
	if err != nil {
		fmt.Println(err)
		return &PaymentResponse{Error: "Error creating request"}
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return &PaymentResponse{Error: "Error sending request"}
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		return &PaymentResponse{Error: "Error reading response body"}
	}

	if !ValidateResponse(string(body), pn.IntegrationKey) {
		return &PaymentResponse{Error: "The response is invalid or has been tampered with."}
	}

	response, err := NewPaymentResponse(string(body))

	if err != nil {
		return &PaymentResponse{Error: err.Error()}
	}

	return response
}

// Assuming the Paynow and Payment structs are defined elsewhere
func (pn *Paynow) SendMobile(p *Payment, phone, method string) *PaymentResponse {

	if len(p.AuthEmail) == 0 {
		return &PaymentResponse{Error: "Auth email is required for mobile transactions"}
	}

	amount := math.Trunc(p.Total()*100) / 100

	if amount <= 0 {
		return &PaymentResponse{Error: "Transaction total cannot be less than 1"}
	}

	// Generate the hash
	hash := GenerateHash(pn.ResultURL, pn.ReturnURL, p.Reference, fmt.Sprintf("%f", p.Total()), pn.IntegrationID, url.QueryEscape(p.Info()), p.AuthEmail, method, phone, "Message", pn.IntegrationKey)

	uri := "https://www.paynow.co.zw/interface/remotetransaction"
	requestMethod := "POST"

	payload := &bytes.Buffer{}
	writer := multipart.NewWriter(payload)

	// Add fields to the writer. Replace hardcoded values with values from the Payment object as needed
	_ = writer.WriteField("resulturl", pn.ResultURL)
	_ = writer.WriteField("returnurl", pn.ReturnURL)
	_ = writer.WriteField("reference", p.Reference)
	_ = writer.WriteField("amount", fmt.Sprintf("%f", amount))
	_ = writer.WriteField("id", pn.IntegrationID)
	_ = writer.WriteField("additionalinfo", url.QueryEscape(p.Info())) // URL encoded
	_ = writer.WriteField("authemail", p.AuthEmail)                    // URL encoded
	_ = writer.WriteField("method", method)
	_ = writer.WriteField("phone", phone)
	_ = writer.WriteField("status", "Message")
	_ = writer.WriteField("hash", hash)

	err := writer.Close()
	if err != nil {
		fmt.Println(err)
		return &PaymentResponse{Error: "Error closing writer"}
	}

	client := &http.Client{}
	req, err := http.NewRequest(requestMethod, uri, payload)
	if err != nil {
		fmt.Println(err)
		return &PaymentResponse{Error: "Error creating request"}
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return &PaymentResponse{Error: "Error sending request"}
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		return &PaymentResponse{Error: "Error reading response body"}
	}

	status := "status=Error&error=Failed+to+initiate+transaction%2c+please+try+later"

	if string(body) == status {
		return &PaymentResponse{Error: "Insufficient Funds"}
	}

	if !ValidateResponse(string(body), pn.IntegrationKey) {
		return &PaymentResponse{Error: "The response is invalid or has been tampered with."}
	}

	response, err := NewPaymentResponse(string(body))

	if err != nil {
		return &PaymentResponse{Error: err.Error()}
	}

	return response
}

// NewPaymentResponse creates a new PaymentResponse instance from a data map.
func NewPaymentResponse(response string) (*PaymentResponse, error) {
	// Parse the query string into a map
	values, err := url.ParseQuery(response)
	if err != nil {
		return nil, err
	}
	// Initialize the response struct
	resp := &PaymentResponse{
		Status: values.Get("status"),
	}

	// Check if it's an error response
	if errMsg := values.Get("error"); errMsg != "" {
		resp.Error = errMsg
	} else {
		// It's a success response, populate the success fields
		resp.BrowserURL, _ = url.QueryUnescape(values.Get("browserurl"))
		resp.PollURL, _ = url.QueryUnescape(values.Get("pollurl"))
		resp.Hash = values.Get("hash")
		if values.Get("authorizationcode") != "" {
			resp.AuthorizationCode = values.Get("authorizationcode")
			resp.AuthorizationExpires = values.Get("authorizationexpires")
		}
	}

	return resp, nil
}

func NewPayment(reference, authEmail string) *Payment {
	return &Payment{
		Reference: reference,
		AuthEmail: authEmail,
	}
}

// Add adds an item to the 'cart'.
func (p *Payment) Add(title string, amount float64) *Payment {
	p.Items = append(p.Items, [2]interface{}{title, amount})
	return p
}

// Total calculates the total cost of the items in the transaction.
func (p *Payment) Total() float64 {
	var total float64
	for _, item := range p.Items {
		amount, _ := item[1].(float64) // Safe type assertion, assuming amount is always float64
		total += amount
	}
	return total
}

// Info generates text which represents the items in cart.
func (p *Payment) Info() string {
	out := ""
	for _, item := range p.Items {
		title, _ := item[0].(string) // Safe type assertion, assuming title is always string
		out += title + ", "
	}
	if len(out) > 0 {
		out = out[:len(out)-2] // Remove trailing comma and space
	}
	return out
}

// GenerateHash creates a SHA512 hash of the concatenated string of payment details.
func GenerateHash(values ...string) string {
	concatenated := strings.Join(values, "")
	hasher := sha512.New()
	hasher.Write([]byte(concatenated))
	hash := hasher.Sum(nil)
	return strings.ToUpper(hex.EncodeToString(hash))
}

// validateResponse takes the response body as a string and validates its hash.
// Replace "YourIntegrationKey" with your actual integration key.
func ValidateResponse(body, integrationKey string) bool {
	parts := strings.Split(body, "&")
	var valuesToHash []string

	// Hash value from the response for comparison
	var hashFromResponse string

	for _, part := range parts {
		keyValue := strings.Split(part, "=")
		if len(keyValue) != 2 {
			fmt.Println("Error: Invalid key-value pair in the response.")
			return false
		}

		key, value := keyValue[0], keyValue[1]
		if key == "hash" {
			hashFromResponse = value
			continue
		}

		decodedValue, err := url.QueryUnescape(value)
		if err != nil {
			fmt.Printf("Error decoding value for %s: %s\n", key, err)
			return false
		}

		valuesToHash = append(valuesToHash, decodedValue)
	}

	// Concatenate all values, including the integration key, and hash the result
	finalStringToHash := strings.Join(valuesToHash, "") + integrationKey
	hash := sha512.Sum512([]byte(finalStringToHash))
	generatedHash := strings.ToUpper(hex.EncodeToString(hash[:]))

	// Compare the generated hash with the hash from the response
	return generatedHash == hashFromResponse
}

// Function to generate the QR code URL using Google Chart API.
func GenerateQRCodeURL(authorizationCode string) string {
	qrCodeURL := "https://chart.googleapis.com/chart?chs=150x150&cht=qr&chl=" + url.QueryEscape(authorizationCode)
	return qrCodeURL
}

// Function to generate the deep link for the InnBucks mobile app.
func GenerateDeepLink(authorizationCode string) string {
	deepLink := "schinn.wbpycode://innbucks.co.zw?pymInnCode=" + url.QueryEscape(authorizationCode)
	return deepLink
}

// FetchPaymentStatus makes a GET request to the specified URL and parses the response into PaymentStatusResponse.
func FetchPaymentStatus(requestURL string) (*PaymentStatusResponse, error) {
	// Make the request
	resp, err := http.Get(requestURL)
	if err != nil {
		return nil, fmt.Errorf("error making GET request: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}

	// Assuming the response is URL-encoded, parse it
	responseValues, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, fmt.Errorf("error parsing response string: %v", err)
	}

	// Manually construct PaymentStatusResponse from parsed values
	statusResponse := &PaymentStatusResponse{
		Reference:       responseValues.Get("reference"),
		PaynowReference: responseValues.Get("paynowreference"),
		PollURL:         responseValues.Get("pollurl"),
		Status:          responseValues.Get("status"),
		Hash:            responseValues.Get("hash"),
	}

	return statusResponse, nil
}

func Poll(requestURL string, maxAttempts int, duration time.Duration) (bool, string) {
	attempts := 0

	// Create a context with a timeout that generously covers the maximum polling duration
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(maxAttempts)*duration*time.Second+10*time.Second)
	defer cancel() // Ensure the context is cancelled to free resources

	var message string // To store the outcome message

	// Poll every 15 seconds for payment status until the context is done, condition is met, or maxAttempts is reached
	result := wait.PollUntilWithContext(ctx, duration*time.Second, func(ctx context.Context) (bool, error) {
		attempts++
		statusResponse, err := FetchPaymentStatus(requestURL)
		if err != nil {
			message = fmt.Sprintf("Error fetching payment status: %v", err)
			return false, err // Stop polling due to error
		}

		fmt.Printf("Attempt %d - Checking payment status: %s\n", attempts, statusResponse.Status)

		if statusResponse.Status == "Paid" {
			message = statusResponse.Status
			return true, nil // Stop polling because payment was successful
		}

		if statusResponse.Status == "Cancelled" {
			message = statusResponse.Status
			return true, nil // Stop polling because payment was cancelled
		}

		if attempts >= maxAttempts {
			message = fmt.Sprintf("Maximum attempts reached without payment confirmation after %d attempts", maxAttempts)
			return true, fmt.Errorf(message) // Stop polling
		}

		// Continue polling
		return false, nil
	})

	// Check result to determine the final return values
	if result == nil {
		return true, message // Polling succeeded without errors
	}
	return false, message // Polling stopped due to an error or condition met
}
