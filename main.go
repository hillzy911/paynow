package main

import (
	"fmt"
)

func main() {
	pn := InitializeSDK("17255", "83a2e4dc-e5e8-4bec-99bb-2c6090000a45", "http://google.com", "http://google.com")
	payment := NewPayment("test", "hchipunza@gmail.com")
	payment.Add("test", 2.0)
	payment.Add("potato", 4.0)
	response := pn.SendMobile(payment, "0771111111", "ecocash")

	if response.AuthorizationCode != "" {
		// Generate QR code URL and deep link
		qrCodeURL := generateQRCodeURL(response.AuthorizationCode)
		deepLink := generateDeepLink(response.AuthorizationCode)
		fmt.Println("QR Code URL:", qrCodeURL)
		fmt.Println("Deep Link:", deepLink)
	}
	fmt.Println(response.BrowserURL)
	fmt.Println(response.PollURL)
	fmt.Println(response.Error)
	fmt.Println("Items:", payment.Info())
}
