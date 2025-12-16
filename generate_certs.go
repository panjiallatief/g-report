// +build ignore

// generate_certs.go - Generates self-signed SSL certificates for local development
// Run: go run generate_certs.go

package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"time"
)

func main() {
	// Generate private key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatalf("Failed to generate private key: %v", err)
	}

	// Create certificate template
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		log.Fatalf("Failed to generate serial number: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization:  []string{"IT Broadcast Ops - Development"},
			Country:       []string{"ID"},
			Province:      []string{"DKI Jakarta"},
			Locality:      []string{"Jakarta"},
			CommonName:    "localhost",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour), // Valid for 1 year
		
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		
		// Add localhost and common IPs for development
		DNSNames:    []string{"localhost", "*.localhost"},
		IPAddresses: []net.IP{
			net.ParseIP("127.0.0.1"),
			net.ParseIP("::1"),
			net.ParseIP("0.0.0.0"),
		},
	}

	// Create certificate
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		log.Fatalf("Failed to create certificate: %v", err)
	}

	// Write certificate to file
	certOut, err := os.Create("server.crt")
	if err != nil {
		log.Fatalf("Failed to create server.crt: %v", err)
	}
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	certOut.Close()
	fmt.Println("✅ Created server.crt")

	// Write private key to file
	keyOut, err := os.Create("server.key")
	if err != nil {
		log.Fatalf("Failed to create server.key: %v", err)
	}
	
	privBytes, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		log.Fatalf("Failed to marshal private key: %v", err)
	}
	pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes})
	keyOut.Close()
	fmt.Println("✅ Created server.key")

	fmt.Println("")
	fmt.Println("🎉 SSL certificates generated successfully!")
	fmt.Println("   The server will now start in HTTPS mode.")
	fmt.Println("")
	fmt.Println("📌 Note: Your browser will show a security warning because")
	fmt.Println("   this is a self-signed certificate. Click 'Advanced' and")
	fmt.Println("   'Proceed to localhost (unsafe)' to continue.")
	fmt.Println("")
	fmt.Println("🚀 Run the server with: go run main.go")
}
