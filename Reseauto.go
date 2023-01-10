package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type Link [2]int

type Interface struct {
	name string
	ip   string
}

type Router struct {
	id         int
	interfaces []string
	ASBR       bool
}

type AS struct {
	ASN     string
	IGP     string
	routers []Router
	adj     [][]int
}

func importLinks(url string) []Link {
	// Importing .json files
	file, _ := os.Open(url)
	defer file.Close()

	// Lecture du .json
	data, err := io.ReadAll(file)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	// On caste le contenu en map de string
	var linksMap map[string]interface{}
	err = json.Unmarshal([]byte(data), &linksMap)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	// On boucle dans la map pour extraire les valeurs et créer un []Link
	var links []Link
	for _, value := range linksMap["links"].([]any) {
		var link Link
		for i, value2 := range value.([]any) {
			link[i] = int(value2.(float64))
		}
		links = append(links, link)
	}

	return links
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
	var ASMap map[string]interface{}
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
		var router Router
		router.id = int(value.(float64))
		as.routers = append(as.routers, router)
	}

	// On initialise la matrice d'adjacence
	as.adj = make([][]int, len(as.routers))
	for i := range as.routers {
		as.adj[i] = make([]int, len(as.routers))
	}

	// On remplit la matrice d'adjacence
	for _, value := range ASMap["links"].([]any) {
		r1 := int(value.([]any)[0].(float64))
		r2 := int(value.([]any)[1].(float64))
		var idr1, idr2 int
		for i, v := range as.routers {
			if v.id == r1 {
				idr1 = i
			}
			if v.id == r2 {
				idr2 = i
			}
		}
		as.adj[idr1][idr2], as.adj[idr2][idr1] = 1, 1
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

func main() {
	links := importLinks("Links.json")
	AS1 := importAS("AS1.json")
	AS2 := importAS("AS2.json")
	fmt.Println("Résultat :", links, AS1, AS2)
}
