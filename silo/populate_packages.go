package main

import (
	"fmt"
	"io/ioutil"
	"sync"

	"github.com/unixvoid/glogger"
	"gopkg.in/redis.v5"
)

func populatePackages(domain, basedir string, redisClient *redis.Client) {
	dirs, _ := ioutil.ReadDir(basedir)
	// make a wait group for concurrency
	var wg sync.WaitGroup

	for _, d := range dirs {
		// dont add 'pubkey', we are not indexing it
		if d.Name() != "pubkey" {
			// up the waitgroup counter
			wg.Add(1)
			glogger.Debug.Printf("adding entry to 'master:packages' :: %s", d.Name())
			err := redisClient.SAdd("master:packages", d.Name()).Err()
			if err != nil {
				glogger.Error.Printf("error adding entry '%s' to master:packages\n", d.Name())
				glogger.Error.Println(err)
			} else {
				// if entry was added to master:packages, generate the metadata line
				go generateMeta(&wg, domain, d.Name(), redisClient)
			}
		}
	}
	// wait for all concurrent processes to complete
	wg.Wait()
	// now that the data has been entered, generate the master metadata header
	glogger.Debug.Println("updating master:metadata")
	packages, err := redisClient.SInter("master:packages").Result()
	if err != nil {
		glogger.Error.Println("error getting master:package for metadata concatination")
		glogger.Error.Println(err)
	}
	for _, pkg := range packages {
		// get current master metadata
		currentMeta, _ := redisClient.Get("master:metadata").Result()
		// get package metadata
		pkgMeta, _ := redisClient.Get(fmt.Sprintf("package:%s", pkg)).Result()
		newMeta := fmt.Sprintf("%s\n%s", currentMeta, pkgMeta)

		err := redisClient.Set("master:metadata", newMeta, 0).Err()
		if err != nil {
			glogger.Error.Printf("error updating master:meatadata with '%s'\n", pkg)
			glogger.Error.Println(err)
		}
	}
}

func generateMeta(wg *sync.WaitGroup, domain, pkg string, redisClient *redis.Client) {
	//aciline := fmt.Sprintf("<meta name=\"ac-discovery\" content=\"%s/%s https://%s/rkt/%s/%s-{version}-{os}-{arch}.{ext}\">", domain, pkg, domain, pkg, pkg)
	//pubkeyline := fmt.Sprintf("<meta name=\"ac-discovery-pubkeys\" content=\"%s/%s https://%s/rkt/pubkey/pubkeys.gpg\">", domain, pkg, domain)
	//metaentry := fmt.Sprintf("%s\n%s", aciline, pubkeyline)

	metaentry := fmt.Sprintf(`<meta name="ac-discovery" content="%s/%s https://%s/rkt/%s/%s-{version}-{os}-{arch}.{ext}">
<meta name="ac-discovery-pubkeys" content="%s/%s https://%s/rkt/pubkey/pubkeys.gpg">`, domain, pkg, domain, pkg, pkg, domain, pkg, domain)

	glogger.Debug.Printf("adding meta line to 'package:%s'", pkg)
	err := redisClient.Set(fmt.Sprintf("package:%s", pkg), metaentry, 0).Err()
	if err != nil {
		glogger.Error.Printf("error adding meta tag to package:%s\n", pkg)
		glogger.Error.Println(err)
	}

	// decrement the wg counter
	defer wg.Done()
}