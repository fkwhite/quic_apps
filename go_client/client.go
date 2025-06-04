package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"os"
	"sync"
	"time"

	xrand "golang.org/x/exp/rand"
	"gonum.org/v1/gonum/stat/distuv"

	quic "github.com/fkwhite/Quic_GO"
	"github.com/mikioh/tcpinfo"
)

type ConfigDistribution struct {
	Type     string  //json:"Distribution Type: fixed,poisson,uniform,LogNormal(ZipFintheFuture)"
	Rate     float64 //json:"Rate (Poisson,LogNormal,fixed) [Mbps]"
	Variance float64 //json:"Variance (LogNormal)"
	Min      float64 //json:"Uniform data Min [Mbps]"
	Max      float64 //json:"Uniform data Max [Mbps]"
}

type ConfigStream struct {
	FileSize     float64            //bytes envia el cliente (en un slot temporal) --> multiplo de bytesStream [MBytes]
	Distribution ConfigDistribution //duracion total del envio [s]
}

type Configuration struct {
	Addr         string
	PktSize      float64 //bytes (estandar Stream)
	TotalSession int
	TotalStream  int
	TimeSlot     int  //duracion slot [ms]
	TCP          bool //TCP or QUIC connection
	InfoStream   []ConfigStream
}

func main() {
	var 
	// time.Sleep(1 * time.Second) //necesario para que le de tiempo al servidor a ejecutar primero
	configFile = os.Args[1]

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

	quic.GlobalBuffersInit(configuration.TotalStream) //inicialización de los buffers

	for {
		err = clientMain(configuration)
		if err == nil {
			break //evitar volver a mandar datos
		}
		fmt.Println("Retry")
	}
}

func clientMain(configuration Configuration) error {
	// Connection TCP or QUIC
	tcp := configuration.TCP

	// wireshark
	var keyLog io.Writer
	f, err := os.Create("key.log")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	keyLog = f

	if tcp {
		// TLS + TCP
		sendDataTCP(configuration)

	} else /* QUIC */ {
		tlsConf := &tls.Config{
			InsecureSkipVerify: true,
			NextProtos:         []string{"quic-echo-example"},
			KeyLogWriter:       keyLog, //wireshark
		}

		var wg sync.WaitGroup //wait for multiple goroutines to finish

		//el cliente puede abrir varias sesiones
		for i := 1; i <= configuration.TotalSession; i++ {
			session, err := quic.DialAddr(configuration.Addr, tlsConf, nil)
			if err != nil {
				return err
			}
			log.Printf("Client : Session %d \n", i)

			// el cliente envia varios streams en cada sesion
			for j := 1; j <= configuration.TotalStream; j++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					sendStream(session, i, configuration)
				}(i)
				//time.Sleep(10 * time.Millisecond)
			}
		}
		wg.Wait()
	}

	return nil
}

func sendStream(session quic.Session, numSession int, configuration Configuration) {
	stream, err := session.OpenStreamSync(context.Background())
	numStream := int(stream.StreamID() / 4)
	if err != nil {
		panic(err)
	}

	nameFile := fmt.Sprint("tmp/logClient_", numSession, "_", numStream+1, ".log") //logClient_numSession_numStream.log
	f, err := os.OpenFile(nameFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println("An error has ocurred -- Opening file")
		panic(err)
	}
	defer f.Close()

	sendData(stream, nil, f, configuration, numStream)
}

// TLS + TCP --> send data
func sendDataTCP(configuration Configuration) {
	// TLS
	cert, err := tls.LoadX509KeyPair("certs/client.pem", "certs/client.key")
	if err != nil {
		panic(err)
	}
	config := tls.Config{Certificates: []tls.Certificate{cert}, InsecureSkipVerify: true}

	conn, err := tls.Dial("tcp", configuration.Addr, &config)
	for err != nil {
		fmt.Println("Retry")
		conn, err = tls.Dial("tcp", configuration.Addr, &config)
	}
	defer conn.Close()

	go congestionWindowTCP()

	//TCP
	nameFile := fmt.Sprint("tmp/logClientTCP", ".log") //logClientTCP.log
	f, err := os.OpenFile(nameFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println("An error has ocurred -- Opening file")
		panic(err)
	}
	defer f.Close()

	//Send data
	sendData(nil, conn, f, configuration, 0)
}

func SetupDistribution(configuration Configuration, numStream int) float64 {

	var numPktperSlot float64

	source := xrand.NewSource(uint64(time.Now().UnixNano()))

	switch configuration.InfoStream[numStream].Distribution.Type {

	case "poisson":
		rateEvents := (((configuration.InfoStream[numStream].Distribution.Rate * 1e6) / 8) / (configuration.PktSize)) * (float64(configuration.TimeSlot) * 1e-3)
		poisson := distuv.Poisson{
			Lambda: rateEvents,
			Src:    source,
		}
		numPktperSlot = poisson.Rand()
	case "fixed":
		numPktperSlot = (((configuration.InfoStream[numStream].Distribution.Rate * 1e6) * (float64(configuration.TimeSlot) * 1e-3)) / 8) / configuration.PktSize
	case "LogNormal":
		rateEvents := (((configuration.InfoStream[numStream].Distribution.Rate * 1e6) / 8) / (configuration.PktSize)) * (float64(configuration.TimeSlot) * 10e-3)
		LogNormal := distuv.LogNormal{
			Mu:    rateEvents,
			Sigma: configuration.InfoStream[numStream].Distribution.Variance,
			Src:   source,
		}
		numPktperSlot = LogNormal.Rand()

	case "uniform":
		Uniform := distuv.Uniform{
			Min: (((configuration.InfoStream[numStream].Distribution.Min * 1e6) / 8) / configuration.PktSize),
			Max: (((configuration.InfoStream[numStream].Distribution.Max * 1e6) / 8) / configuration.PktSize),
			Src: source,
		}

		numPktperSlot = Uniform.Rand()

	default:
		fmt.Println("Error: Distribution")
	}

	return float64(numPktperSlot)
}

func sendData(stream quic.Stream, conn net.Conn, f *os.File, configuration Configuration, numStream int) {
	fileSizeStream := configuration.InfoStream[numStream].FileSize * 1e6
	pktLen := configuration.PktSize
	fmt.Println("Sending stream...", numStream)

	quic.GlobalBuffersWrite(numStream, 0.0)
	rem := fileSizeStream
	// if (!configuration.TCP){
	// 	go DoSendData(stream, &rem, pktLen, configuration, numStream)
	// }
	
	ticker := time.NewTicker(time.Duration(configuration.TimeSlot) * time.Millisecond) //tiempo que se repite --> slot
	bytesSentperStream := make([]int, configuration.TotalStream)
	timestampBeg := time.Now().UnixMicro()
	
	for range ticker.C {
		numPktperSlot := math.Round(SetupDistribution(configuration, numStream))		
		newBytes := math.Min(numPktperSlot*pktLen, rem)
		bytesSentperStream[numStream] += int(newBytes)
		// buffBytes := quic.GlobalBuffersRead(numStream)
		
		quic.GlobalBuffersIncr(numStream, newBytes)
		rem -= newBytes
		// fmt.Printf("Stream %d: sent %d bytes, rem %d\n", numStream, int(newBytes), int(rem))
		// buffBytes = quic.GlobalBuffersRead(numStream)
		// fmt.Printf("Stream %d: sent %d bytes [queue: %d] \n", numStream, int(newBytes), int(buffBytes))

		
		aux := newBytes
		for aux > 0 {
			timestamp := time.Now().UnixMicro()
			if configuration.TCP {
				sent := math.Min(pktLen, aux)
				message := make([]byte, int(sent))
				_, err := conn.Write(message)
				if err != nil {
					panic(err)
				}
				logMessage := fmt.Sprint(timestamp, "	", sent, "\n")
				_, err = f.WriteString(logMessage) //fichero log
				if err != nil {
					fmt.Println("An error has ocurred -- Writing file")
					panic(err)
				}
				aux -= sent
			}else{
				toSend := math.Min(quic.GlobalBuffersRead(numStream), pktLen)
				message := make([]byte, int(toSend))
				_, err := stream.Write(message) 
				if err != nil {
					panic(err)
				}
				quic.GlobalBuffersIncr(numStream, -toSend)
				logMessage := fmt.Sprint(timestamp, "	", toSend, "\n")
				_, err = f.WriteString(logMessage) //fichero log
				if err != nil {
					fmt.Println("An error has ocurred -- Writing file")
					panic(err)
				}
				aux -= toSend

			}
		}
		if rem == 0 {
			break
		}
	}

	timestampEnd := time.Now().UnixMicro()
	ticker.Stop()
	elapsedTime := (timestampEnd - timestampBeg)
	rate := float64(fileSizeStream) * 8 / float64(elapsedTime)
	fmt.Printf("----- Stream %d finished: %d MB sent at %f Mbps\n", numStream, int(fileSizeStream*1e-6), rate)
	if (!configuration.TCP){
		for {
			buffBytes := quic.GlobalBuffersRead(numStream)
			fmt.Printf("Stream %d: waiting with queue %d ... \n", numStream, int(buffBytes))
			time.Sleep(1000 * time.Millisecond)
			if buffBytes == 0.0 {
				break
			}
		}
	}

	if(!configuration.TCP){
		quic.GlobalBuffersLog(numStream)
	}
	
}

// func DoSendData(stream quic.Stream, rem *float64, pktLen float64, configuration Configuration, numStream int) {
// 	ctr := 0.0
// 	var err error
// 	for quic.GlobalBuffersRead(numStream) > 0.0 || *rem > 0.0 {

// 		toSend := math.Min(quic.GlobalBuffersRead(numStream), pktLen)
// 		ctr += toSend
		
// 		// fmt.Printf("Stream %d Data to send = %d rem = %d BufferData = %d\n", numStream, int(toSend), int(*rem),int(quic.GlobalBuffersRead(numStream)))

// 		message := make([]byte, int(toSend))

// 		_, err = stream.Write(message) 
		

// 		if err != nil {
// 			panic(err)
// 		}
// 		quic.GlobalBuffersIncr(numStream, -toSend)
// 	}
// 	fmt.Printf("+++++++++++++++++++++++ Stream %d DoSend is done with %f bytes!!\n", numStream, ctr)

// }


func congestionWindowTCP() {
	fmt.Println("Enter congestionWindowTCP")
	var cwndTCP tcpinfo.CongestionControl
	cwnd := 1
	cwndFile := fmt.Sprint("LOGS_congestionWindow/congestionTCP.log") //logClientTCP.log
	fTCP, err := os.OpenFile(cwndFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println("An error has ocurred -- Opening file")
		panic(err)
	}
	defer fTCP.Close()

	for {
		congestionWindow := int(cwndTCP.SenderWindowBytes)
		if cwnd != congestionWindow {
			fmt.Println("Enter IF")
			cwnd = congestionWindow
			_, err := fTCP.WriteString(fmt.Sprint(congestionWindow, "\n")) //fichero log
			if err != nil {
				fmt.Println("An error has ocurred -- Writing file")
				panic(err)
			}
		}
	}

}
