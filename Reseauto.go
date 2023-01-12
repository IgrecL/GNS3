package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

type IP struct {
	digits [8]int
	mask   int
}

func (ip IP) toString() string {
	var str string
	for i := 0; i < 8; i++ {
		str += strconv.FormatInt(int64(ip.digits[i]), 16) + ":"
	}
	str = str[:len(str)-1]
	str += "/" + fmt.Sprint(ip.mask)
	return str
}

func (ip *IP) toInt(ipString string) {
	split := strings.Split(ipString, ":")
	if len(split) < 8 {
		var splitComplet [8]string
		var stop bool
		var i, j int = 0, 0
		for ; !stop; i++ {
			splitComplet[i] = split[i]
			if split[i] == "" {
				stop = true
			}
		}
		stop = false
		for ; !stop; j++ {
			splitComplet[7-j] = split[len(split)-1-j]
			if split[len(split)-1-j] == "" {
				stop = true
			}
		}
		split = splitComplet[:]
	}
	split2 := strings.Split(split[7], "/")
	split[7] = split2[0]
	ip.mask, _ = strconv.Atoi(split2[1])
	for i, s := range split {
		numHex, _ := strconv.ParseInt(s, 16, 64)
		ip.digits[i] = int(numHex)
	}
}

func (ip IP) increment() IP {
	var new IP
	for i, v := range ip.digits {
		new.digits[i] = v
	}
	new.mask = ip.mask
	for i := 7; i >= 0; i-- {
		if new.digits[i] == 65535 {
			new.digits[i] = 0
		} else {
			new.digits[i]++
			break
		}
	}
	return new
}

func (ip1 IP) equals(ip2 IP) bool {
	for i := 0; i < 8; i++ {
		if ip1.digits[i] != ip2.digits[i] {
			return false
		}
	}
	return true
}

type Interface struct {
	name     string
	ip       IP
	idRouter int
	ASBR     bool
}

type Link [2]Interface

type AS struct {
	ASN       int
	IGP       string
	routersId []int
	adj       [][]*Link
}

func importGlobal(url string) ([]Link, [2]IP) {
	// Importing .json files
	file, _ := os.Open(url)
	defer file.Close()

	// Lecture du .json
	data, err := io.ReadAll(file)
	if err != nil {
		fmt.Println(err)
		var nilArray [2]IP
		return nil, nilArray
	}

	// On caste le contenu en map de string
	var linksMap map[string]interface{}
	err = json.Unmarshal([]byte(data), &linksMap)
	if err != nil {
		fmt.Println(err)
		var nilArray [2]IP
		return nil, nilArray
	}

	// On récupère la pool d'IP
	var ipRange [2]IP
	ipRange[0].toInt(linksMap["ip_range"].([]any)[0].(string))
	ipRange[1].toInt(linksMap["ip_range"].([]any)[1].(string))

	// On boucle dans la map pour extraire les valeurs et créer un []Link
	var links []Link
	for _, value := range linksMap["links"].([]any) {
		var link Link
		for i, value2 := range value.([]any) {
			var in Interface
			in.idRouter = int(value2.(map[string]any)["id"].(float64))
			in.name = value2.(map[string]any)["interface"].(string)
			in.ASBR = true
			link[i] = in
		}
		links = append(links, link)
	}

	return links, ipRange
}

func importAS(url string) AS {
	// Importing .json files
	file, _ := os.Open(url)
	defer file.Close()

	// Lecture du .json
	data, err := io.ReadAll(file)
	if err != nil {
		fmt.Println(err)
		return *new(AS)
	}

	// On caste le contenu en map de string
	var ASMap map[string]any
	err = json.Unmarshal([]byte(data), &ASMap)
	if err != nil {
		fmt.Println(err)
		return *new(AS)
	}

	// On importe le protocole IGP utilisé
	var as AS
	as.IGP = ASMap["protocol"].(string)

	// On importe la liste des routeurs de l'AS
	for _, value := range ASMap["routers"].([]any) {
		as.routersId = append(as.routersId, int(value.(float64)))
	}

	// On initialise la matrice d'adjacence
	as.adj = make([][]*Link, len(as.routersId))
	for i := range as.routersId {
		as.adj[i] = make([]*Link, len(as.routersId))
	}

	// On remplit la matrice d'adjacence
	for _, value := range ASMap["links"].([]any) {

		// On récupère les id et les noms d'interface
		var int1, int2 Interface
		int1.idRouter = int(value.([]any)[0].(map[string]any)["id"].(float64))
		int2.idRouter = int(value.([]any)[1].(map[string]any)["id"].(float64))
		int1.name = value.([]any)[0].(map[string]any)["interface"].(string)
		int2.name = value.([]any)[1].(map[string]any)["interface"].(string)
		var link Link
		link[0], link[1] = int1, int2

		// On récupère l'indice du routeur dans la liste de routeurs
		var index1, index2 int
		for i, id := range as.routersId {
			if id == int1.idRouter {
				index1 = i
			}
			if id == int2.idRouter {
				index2 = i
			}
		}

		// On remplit la matrice d'adjacence de liens
		as.adj[index1][index2] = &link
	}

	return as
}

func printMat(M [][]int) {
	for _, v := range M {
		for _, w := range v {
			fmt.Print(w, " ")
		}
		fmt.Println()
	}
}

func giveIP(ASList []AS, ipRange [2]IP) {
	ipMin, ipMax := ipRange[0], ipRange[1]
	for _, AS := range ASList {
		for i := 0; i < len(AS.routersId); i++ {
			for j := 0; j < i; j++ {
				if AS.adj[i][j] != nil {
					for k := 0; k < 2; k++ {
						if ipMin.equals(ipMax.increment()) {
							fmt.Println("L'IP maximale a été atteinte !")
							return
						}
						(AS.adj[i][j])[k].ip = ipMin
						fmt.Println((AS.adj[i][j])[k].ip.toString())
						ipMin = ipMin.increment()
					}
				}
			}
		}
	}
}

func main() {

	// On importe le contenu des fichiers .json
	global, ipRange := importGlobal("Global.json")
	var ASList []AS
	ASList = append(ASList, importAS("AS1.json"))
	ASList = append(ASList, importAS("AS2.json"))

	// On assigne les ASN aux AS
	for i := 0; i < len(ASList); i++ {
		ASList[i].ASN = i
	}

	// On attribue les adresses IP parmi celles du range de Global.json
	giveIP(ASList, ipRange)

	global, ipRange, ASList = global, ipRange, ASList

	// output := "salut"
	// if err := os.WriteFile("output.ios", []byte(output), 0666); err != nil {
	// 	log.Fatal(err)
	// }
}
