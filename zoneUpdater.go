package main

import (
	"fmt"
	"github.com/mitchellh/goamz/route53"
	"log"
)

type ZoneUpdater struct {
	AwsClient  *route53.Route53
	HostedZone string
	UpdatesCh  chan *route53.Change
}

// Process updates, updating records sets in AWS Route53
func (z *ZoneUpdater) listen() {
	for change := range z.UpdatesCh {
		resourceRecords := z.getResourceRecords(change.Record.Name)
		if fmt.Sprintf("%v", resourceRecords) != fmt.Sprintf("%v", change.Record.Records) {
			log.Printf("-- ZONEUPDATER:%s:updating:%s:%s\n", z.HostedZone, change.Record.Name, change.Record.Records)
			req := &route53.ChangeResourceRecordSetsRequest{
				Comment: "lbManager",
				Changes: []route53.Change{*change},
			}
			_, err := z.AwsClient.ChangeResourceRecordSets(z.HostedZone, req)
			if err != nil {
				log.Println(err)
			}
		} else {
			log.Printf("-- ZONEUPDATER:%s:nothingToUpdate", z.HostedZone)
		}
	}
}

// Get resource records from Route53
func (z *ZoneUpdater) getResourceRecords(name string) (resourceRecords []string) {
	lopts := &route53.ListOpts{
		Name:     name,
		MaxItems: 1,
	}
	resp, err := z.AwsClient.ListResourceRecordSets(z.HostedZone, lopts)
	if err != nil {
		log.Println(err)
	} else {
		if resp.Records[0].Name == name+"." {
			resourceRecords = resp.Records[0].Records
		}
	}
	return
}
