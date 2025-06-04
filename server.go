package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"math/big"
	"os"
	"sync"
	"time"

	// "os/exec"

	quic "github.com/fkwhite/Quic_GO"
)

/* global variable declaration */
var (
	mu            sync.Mutex // guards balance
	bytesReceived int
)

type ConfigStream struct {
	FileSize float64 //bytes envia el cliente (en un slot temporal) --> multiplo de bytesStream [MBytes]
}

type Configuration struct {
	Addr         string
	PktSize      int //bytes (estandar Stream)
	TotalSession int
	TotalStream  int
	TCP          bool //TCP or QUIC connection
	// Distribution  string    //distribucion (uniforme o poisson)
	InfoStream []ConfigStream
}

func main() {
	fmt.Println("Server: Start")

	configFile := os.Args[1]

	file, err := os.Open(configFile)
	if err != nil {
		fmt.Println("An error has ocurred -- Opening configuration file")
		panic(err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	configuration := Configuration{}
	err = decoder.Decode(&configuration)
	if err != nil {
		fmt.Println("error:", err)
	}

	// Connection TCP or QUIC
	tcp := configuration.TCP

	if tcp {
		//TLS + TCP
		readDataTCP(configuration)

	} else /* QUIC */ {
		listener, err := quic.ListenAddr(configuration.Addr, generateTLSConfig(), nil)
		if err != nil {
			fmt.Println("An error has ocurred -- Listening")
			panic(err)
		}
		fmt.Println("Server: Listening")

		var wg sync.WaitGroup //wait for multiple goroutines to finish

		// createSession(listener, 0, configuration)

		//no abrir mas sesiones de las necesarias
		for i := 1; i <= configuration.TotalSession; i++ {
			wg.Add(1)
			go func(numSession int) {
				defer wg.Done()
				createSession(listener, numSession, configuration)
			}(i)
		}

		wg.Wait()
	}
	// out, err := exec.Command("pwd").Output() //sudo killall /go_client/client"
	// if err != nil {
	// 		log.Fatal(err)
	// }
	// fmt.Println(string(out))
	fmt.Println("Server: End")
}

func createSession(listener quic.Listener, numSession int, configuration Configuration) {
	var wg sync.WaitGroup //wait for multiple goroutines to finish

	sess, err := listener.Accept(context.Background())
	if err != nil {
		panic(err)
	}
	bytesReceived = 0

	//no aceptar mas streams de las necesarios
	for i := 1; i <= configuration.TotalStream; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			acceptStream(sess, numSession, configuration)
		}()
	}

	wg.Wait()

	fmt.Println("End Session")
}

func acceptStream(sess quic.Session, numSession int, configuration Configuration) {
	stream, err := sess.AcceptStream(context.Background())
	numStream := int(stream.StreamID() / 4)

	if err != nil {
		fmt.Println("An error has ocurred -- Stream")
		panic(err)
	}
	// sess.GetCongestionWindow()

	nameFile := fmt.Sprint("tmp/logServer_", numSession, "_", numStream+1, ".log") //logServer_numSession_numStream.log
	f, err := os.OpenFile(nameFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println("An error has ocurred -- Opening file")
		panic(err)
	}
	defer f.Close()

	//leer todo el rato de 1 stream (de configuration.PktSize en configuration.PktSize)
	buf := make([]byte, configuration.PktSize)

	fileSize := configuration.InfoStream[numStream].FileSize * 1e6

	timestampBeg := time.Now().UnixMicro()
	first := true
	localBytesReceived := 0

	for localBytesReceived < int(fileSize) {
		numBytes, _ := io.ReadFull(stream, buf)
		timestamp := time.Now().UnixMicro()
		if first {
			timestampBeg = timestamp
			first = false
		}
		localBytesReceived += numBytes
		//fmt.Printf("Stream %d received %d/%d bytes   \n", numStream, localBytesReceived, int(fileSize))

		logMessage := fmt.Sprint(timestamp, "	", numBytes, "\n")
		_, err := f.WriteString(logMessage) //fichero log
		if err != nil {
			fmt.Println("An error has ocurred -- Writing file")
			panic(err)
		}
	}
	timestampEnd := time.Now().UnixMicro()
	elapsedTime := (timestampEnd - timestampBeg)
	rate := float64(localBytesReceived) * 8 / float64(elapsedTime)

	fmt.Printf("Stream ID: %d, received %d MB at %f Mbps\n", numStream, localBytesReceived/1e6, rate)
	fmt.Printf("Stream ID: %d, Total Time: %d s\n", numStream, elapsedTime/1e6)
	// os.Exit(0)
}

// TLS + TCP --> receive data
func readDataTCP(configuration Configuration) {
	fmt.Println("Server: TCP")
	//TLS
	cert, err := tls.LoadX509KeyPair("go_client/certs/server.pem", "go_client/certs/server.key")
	if err != nil {
		log.Fatalf("server: loadkeys: %s", err)
	}
	config := tls.Config{Certificates: []tls.Certificate{cert}}
	config.Rand = rand.Reader

	//TCP
	listener, err := tls.Listen("tcp", configuration.Addr, &config)
	if err != nil {
		fmt.Println("error:", err)
	}

	conn, err := listener.Accept()
	if err != nil {
		fmt.Println("error:", err)
	}

	//TCP
	nameFile := fmt.Sprint("tmp/logServerTCP", ".log") //logServerTCP.log
	f, err := os.OpenFile(nameFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println("An error has ocurred -- Opening file")
		panic(err)
	}
	defer f.Close()

	// Read data
	buf := make([]byte, configuration.PktSize)
	numBytesTotalTCP := 0
	fileSize := configuration.InfoStream[0].FileSize

	for numBytesTotalTCP < int(fileSize*1e6) {
		numBytes, _ := conn.Read(buf)
		timestamp := time.Now().UnixMicro()

		numBytesTotalTCP += numBytes

		if numBytes == 0 {
			//si no hay mas datos, evitar escribir --> timeout
			continue
		}
		logMessage := fmt.Sprint(timestamp, "	", numBytes, "\n")
		//fmt.Println(logMessage) //por pantalla
		_, err := f.WriteString(logMessage) //fichero log
		if err != nil {
			fmt.Println("An error has ocurred -- Writing file")
			panic(err)
		}
	}

	fmt.Println(numBytesTotalTCP)
}

// Setup a bare-bones TLS config for the server
func generateTLSConfig() *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   []string{"quic-echo-example"},
	}
}
