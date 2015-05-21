package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"reflect"
	"sync"
)

type PairKV struct {
	key string
	val interface{}
}

var (
	j1Res, j2Res map[string]int
)

func main() {
	// Read file
	jsonBytes1, err := ioutil.ReadFile("json_1.json")
	if err != nil {
		log.Fatalf("JSON file read error: %v", err)
	}
	jsonBytes2, err := ioutil.ReadFile("json_2.json")
	if err != nil {
		log.Fatalf("JSON file read error: %v", err)
	}
	// log.Printf("%v", jsonBytes

	// Parse JSON
	var j1Raw, j2Raw interface{}
	if err := json.Unmarshal(jsonBytes1, &j1Raw); err != nil {
		log.Fatalf("JSON 1 parsing error: %v", err)
	}
	if err := json.Unmarshal(jsonBytes2, &j2Raw); err != nil {
		log.Fatalf("JSON 2 parsing error: %v", err)
	}

	if err := parseArbitraryJSON(j1Raw); err != nil {
		log.Fatalf("Preparse first JSON file error: ", err)
	}
	fmt.Printf("\n\n")
	if err := parseArbitraryJSON(j2Raw); err != nil {
		log.Fatalf("Preparse second JSON file error: ", err)
	}
	fmt.Printf("\n\n")

	direct := make(chan PairKV)
	back := make(chan string)
	done := make(chan bool)
	j1Res = make(map[string]int)
	j2Res = make(map[string]int)

	wg := sync.WaitGroup{}
	wg.Add(1)

	fmt.Println("Processing...")
	go firstJSON("", j1Raw, direct, back, done, &wg)
	go secondJSON("", j2Raw, direct, back, done)
	wg.Wait()
	// fmt.Printf("%v", j1Res)

	j1ResultStr := ""
	j2ResultStr := ""
	outputJSON("", j1Raw, &j1ResultStr, 0, j1Res)
	outputJSON("", j2Raw, &j2ResultStr, 0, j2Res)

	html := ""
	html += "<html>"
	html += `<head><link rel="stylesheet" href="style.css"></head>`
	html += `<body>` + `<section>` + j1ResultStr + `</section>` + `<section>` + j2ResultStr + `</section>` + `</body>`
	html += `</html>`

	fmt.Println(html)
	if err := ioutil.WriteFile("index.html", []byte(html), 0644); err != nil {
		log.Fatalf("Write html file error: %v", err)
	}
	fmt.Scanln()
}

func outputJSON(parent string, f interface{}, str *string, depth int, colorMap map[string]int) {
	m := f.(map[string]interface{})
	for k, v := range m {
		color := "black"
		if col, ok := colorMap[parent+"/"+k]; ok {
			switch col {
			case 0:
				color = "red"
			case 1:
				color = "yellow"
			case 2:
				color = "green"
			}
		}
		switch v.(type) {
		case string, float64, bool:
			if color == "black" {
				color = "red"
			}
			*str += `<p class="` + color + `" style="margin-left:` + fmt.Sprintf("%v", depth*10) + `px;">` + fmt.Sprintf("%v: %v", k, v) + `</p>`
		case interface{}:
			*str += `<p class="` + color + `" style="margin-left:` + fmt.Sprintf("%v", depth*10) + `px">` + fmt.Sprintf("%v:{", k) + `</p>`
			d1 := depth + 1
			outputJSON(parent+"/"+k, v, str, d1, colorMap)
		default:
			continue
		}
	}
	*str += `<p class="black"` + ` style="margin-left:` + fmt.Sprintf("%v", depth*10-10) + `px;">` + fmt.Sprintf("}") + `</p>`
}

func parseArbitraryJSON(f interface{}) error {
	m := f.(map[string]interface{})
	for k, v := range m {
		switch vv := v.(type) {
		case string:
			fmt.Printf("%v IS string --> %q\n", k, vv)
		case float64:
			fmt.Printf("%v IS number --> %v\n", k, vv)
		case bool:
			fmt.Printf("%v IS bool --> %v\n", k, vv)
		case []interface{}:
			fmt.Printf("%v IS an array:\n", k)
			for _, u := range vv {
				parseArbitraryJSON(u)
			}
		case interface{}:
			fmt.Printf("%v IS an JSON object\n", k)
			parseArbitraryJSON(v)
		default:
			fmt.Printf("Unknown type: %v\n", k)
			return fmt.Errorf("Unknown format: %v", k)
		}
	}
	return nil
}

func firstJSON(parent string, j1Raw interface{}, direct chan PairKV, back chan string, done chan bool, wg *sync.WaitGroup) {
	j1 := j1Raw.(map[string]interface{})
	fmt.Printf("firstJSON entry for %v\n", parent)
	for k, v := range j1 {
		// fmt.Printf("JSON1 key: %v\n", k)
		direct <- PairKV{key: k, val: v}
		com := <-back
		switch com {
		case "notFound":
			fmt.Printf("%v not found in second JSON file\n", k)
			j1Res[parent+"/"+k] = 0
			continue
		case "diffTypes":
			fmt.Printf("Diff types for: %v\n", k)
			j1Res[parent+"/"+k] = 1
		case "diffValues":
			fmt.Printf("Diff values for: %v\n", k)
			j1Res[parent+"/"+k] = 1
		case "eqValues":
			fmt.Printf("Equal values for: %v\n", k)
			j1Res[parent+"/"+k] = 2
		case "in":
			firstJSON(parent+"/"+k, v, direct, back, done, wg)
		default:
			continue
		}
	}
	done <- true
	if parent == "" {
		wg.Done()
	}
}

func secondJSON(parent string, j2Raw interface{}, direct chan PairKV, back chan string, done chan bool) {
	j2 := j2Raw.(map[string]interface{})
	for {
		select {
		case pair := <-direct:
			k := pair.key
			j1Val := pair.val
			j2Val, ok := j2[pair.key]
			// fmt.Printf("%v\n", ok)
			if !ok {
				back <- "notFound"
				continue
			}
			if reflect.TypeOf(j1Val) != reflect.TypeOf(j2Val) {
				back <- "diffTypes"
				j2Res[parent+"/"+k] = 1
				continue
			}
			switch j1Val.(type) {
			case string, float64, bool:
				if j1Val == j2Val {
					j2Res[parent+"/"+k] = 2
					back <- "eqValues"
				} else {
					j2Res[parent+"/"+k] = 1
					back <- "diffValues"
				}
			case interface{}:
				back <- "in"
				secondJSON(parent+"/"+pair.key, j2Val, direct, back, done)
			case []interface{}:
				back <- "diffValues"
				continue // TODO: Array comparing
			default:
				if j1Val == j2Val {
					j2Res[parent+"/"+k] = 2
					back <- "eqValues"
				} else {
					j2Res[parent+"/"+k] = 1
					back <- "diffValues"
				}
			}
		case <-done:
			return
		}
	}
}
