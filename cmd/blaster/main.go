package main

import (
	"fmt"
	//	"io/ioutil"
	"net/http"
	//	"os"
	"container/list"
	"runtime"
	"sync/atomic"
	"time"
)

type messageType int

const (
	OTP         messageType = 1
	NEW_MATCHES             = 2
	ACCEPT                  = 3
)

type message struct {
	msgType      messageType
	text         string
	mobileNumber int
}

type vendor struct {
	id        int
	name      string
	budget    uint64
	utilized  uint64
	serverURL string
}

var acl = &vendor{
	id:        1,
	name:      "ACL",
	budget:    20,
	utilized:  0,
	serverURL: "http://127.0.0.1:8081/",
}

var valueFirst = &vendor{
	id:        2,
	name:      "ValueFirst",
	budget:    1,
	utilized:  0,
	serverURL: "http://127.0.0.1:8082/",
}

var twilio = &vendor{
	id:        2,
	name:      "Twilio",
	budget:    10,
	utilized:  0,
	serverURL: "http://127.0.0.1:8083/",
}

type vendorPercentage struct {
	v       *vendor
	percent float32
}

type messageTypePreference struct {
	vendors []vendorPercentage
}

var messageTypePreferences = map[messageType]*messageTypePreference{
	OTP: &messageTypePreference{
		vendors: []vendorPercentage{
			{
				acl,
				50.0,
			},
			{
				valueFirst,
				50.0,
			},
		},
	},
	NEW_MATCHES: &messageTypePreference{
		vendors: []vendorPercentage{
			{
				acl,
				70.0,
			},
			{
				valueFirst,
				30.0,
			},
		},
	},
	ACCEPT: &messageTypePreference{
		vendors: []vendorPercentage{
			{
				twilio,
				100.0,
			},
		},
	},
}

func utilizeBudget(v *vendor) bool {
	var t, n uint64

	var available bool
	for {
		t = v.utilized

		available = false
		if t < v.budget {
			n = t + 1
			available = true
		} else {
			n = t
		}

		if atomic.CompareAndSwapUint64(&v.utilized, t, n) {
			//fmt.Printf("blocking vendor:%s utilized:%d\n", v.name, v.utilized)
			return available
		}
	}

	return available
}

var queued list.List

func vendorSelection(msg *message) (*vendor, error) {

	//fetch vendors based on preference and budget availability
	pref, ok := messageTypePreferences[msg.msgType]

	if !ok {
		return nil, fmt.Errorf("InvalidMessage msgType:%d\n", msg.msgType)
	}

	var vp vendorPercentage
	found := false
	for _, vp = range pref.vendors {

		if utilizeBudget(vp.v) {
			found = true
			break
		}
	}
	if found {
		//fmt.Printf("selecting vendor:%s utilized:%d\n", vp.v.name, vp.v.utilized)
		return vp.v, nil
	} else {
		return nil, nil
	}
}

type fireMessage struct {
	msg *message
	v   *vendor
}

func fire(f *fireMessage) {
	res, err := http.Get(f.v.serverURL + "manoj")

	if err != nil {
		fmt.Println("request err:", err)
		atomic.SwapUint64(&f.v.utilized, f.v.utilized-1)
		return
	}

	defer res.Body.Close()
	newval := f.v.utilized - 1
	if newval >= f.v.budget+99999999 {
		newval = 0
	}
	atomic.SwapUint64(&f.v.utilized, newval)
	//fmt.Printf("request msg:%d utilized:%d done\n", f.msg.mobileNumber, f.v.utilized)
}

func main() {
	//default to number of cpu cores
	x := runtime.NumCPU()

	noOfProcs := runtime.GOMAXPROCS(x)
	fmt.Printf("No of Processors: %d\n", noOfProcs)

	done := make(chan bool)
	defer close(done)

	messageChan := make(chan *message)
	defer close(messageChan)

	queuedChan := make(chan *message)
	defer close(queuedChan)

	//message generator
	go func() {
		fmt.Println("sms generator started.")
		for i := 0; i < 100000; i++ {
			//fmt.Println("send:", i)
			messageChan <- &message{
				msgType:      OTP,
				text:         fmt.Sprintf("%d have you gone nuts. this might work though.", i),
				mobileNumber: i,
			}
		}
		fmt.Println("sms generator ended.")
	}()

	//message dipatcher
	go func() {
		fmt.Println("dispatcher started.")
		for {
			select {
			case msg := <-messageChan:

				v, _ := vendorSelection(msg)

				if v != nil {
					go fire(&fireMessage{
						msg: msg,
						v:   v,
					})
					//fmt.Printf("fired msg:%d vendor:%s utilized:%d\n", msg.mobileNumber, v.name, v.utilized)
				} else {
					//fmt.Printf("queued msg:%d acl:%d vf:%d\n", msg.mobileNumber, acl.utilized, valueFirst.utilized)
					queuedChan <- msg
				}

			}
		}
		fmt.Println("dispatcher ended.")
	}()

	//queued retry
	go func() {
		fmt.Println("queued started.")
		for {
			select {
			case msg := <-queuedChan:
				//fmt.Println("queued retry ", msg.mobileNumber)
				for {
					v, _ := vendorSelection(msg)

					if v != nil {
						go fire(&fireMessage{
							msg: msg,
							v:   v,
						})
						break
					} else {
						//fmt.Printf("retry full msgType:%d mobile:%d acl:%d vf:%d\n", msg.msgType, msg.mobileNumber, acl.utilized, valueFirst.utilized)
						time.Sleep(10 * time.Millisecond)
						//queuedChan <- msg
					}
				}
			}
		}

		fmt.Println("response ended.")
	}()

	time.Sleep(1 * time.Minute)
	fmt.Println("the end")
}
