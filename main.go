package main

import (
	"fmt"
)

func main() {
	pn := InitializeSDK("17249", "42fc342d-a4d6-4cce-8c00-b1ffc761d0e7", "http://google.com", "http://google.com")
	payment := NewPayment("test", "hchipunza@gmail.com")
	payment.Add("test", 2.0)
	payment.Add("potato", 4.0)
	response := pn.SendMobile(payment, "0774586659", "innbucks")

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
