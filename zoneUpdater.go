package main

import (
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
		log.Printf("-- ZONEUPDATER:%s:updating:%s:%s\n", z.HostedZone, change.Record.Name, change.Record.Records)
		req := &route53.ChangeResourceRecordSetsRequest{
			Comment: "lbManager",
			Changes: []route53.Change{*change},
		}
		_, err := z.AwsClient.ChangeResourceRecordSets(z.HostedZone, req)
		if err != nil {
			log.Println(err)
		}
	}
}
