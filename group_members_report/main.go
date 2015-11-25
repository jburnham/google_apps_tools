package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/admin/directory/v1"
)

// Should be set by ldflags:
// godep go build -ldflags "-X main.gitVersion=$(git rev-parse --short HEAD)"
var gitVersion string

var (
	credentialsFileFlag   = flag.String("credentials-file", "REQUIRED", "The json file from Google that contains the service account private material.")
	impersonatedEmailFlag = flag.String("impersonated-email", "REQUIRED", "The admin user email to impersonate for access.")
	domainFlag            = flag.String("domain", "REQUIRED", "The domain to query for groups.")
	outputFile            = flag.String("output-file", "report.csv", "The csv file to write out.")
	versionFlag           = flag.Bool("version", false, "Show version information.")
)

func main() {
	flag.Parse()

	if *versionFlag {
		fmt.Println("group_members_report", gitVersion)
		os.Exit(0)
	}

	if *credentialsFileFlag == "REQUIRED" || *impersonatedEmailFlag == "REQUIRED" || *domainFlag == "REQUIRED" {
		flag.Usage()
		os.Exit(1)
	}

	file, err := os.Open(*credentialsFileFlag)
	if err != nil {
		log.Fatalf("Could not open file: %v", err)
	}
	service := getAdminService(*impersonatedEmailFlag, file)
	log.Println("Starting report generation")
	groups, err := fetchGroups(service, *domainFlag)
	if err != nil {
		log.Fatalf("Error fetching groups: %v", err)
	}

	rows := [][]string{
		{"group", "email"},
	}

	for _, group := range groups {
		members, err := fetchGroupMembers(service, group)
		if err != nil {
			log.Fatalf("Error fetching group members: %v", err)
		}
		for _, member := range members {
			row := []string{group.Email, member.Email}
			rows = append(rows, row)
		}
	}

	file, err = os.Create(*outputFile)
	if err != nil {
		log.Fatalln("Could not open file for writing: %v", err)
	}
	writer := csv.NewWriter(file)
	err = writer.WriteAll(rows)
	if err != nil {
		log.Fatalf("Error writing csv file: %v", err)
	}
	log.Println("Complete")
}

func getAdminService(adminEmail string, credentialsReader io.Reader) *admin.Service {
	data, err := ioutil.ReadAll(credentialsReader)
	if err != nil {
		log.Fatalf("Can't read Google credentials file: %v", err)
	}
	conf, err := google.JWTConfigFromJSON(data, admin.AdminDirectoryUserReadonlyScope, admin.AdminDirectoryGroupReadonlyScope)
	if err != nil {
		log.Fatalf("Can't load Google credentials file: %v", err)
	}
	conf.Subject = adminEmail

	client := conf.Client(oauth2.NoContext)
	adminService, err := admin.New(client)
	if err != nil {
		log.Fatal(err)
	}
	return adminService
}

func fetchGroups(service *admin.Service, domain string) ([]*admin.Group, error) {
	groups := []*admin.Group{}
	pageToken := ""
	for {
		req := service.Groups.List().Domain(domain)
		if pageToken != "" {
			req.PageToken(pageToken)
		}
		r, err := req.Do()
		if err != nil {
			return nil, err
		}
		for _, group := range r.Groups {
			groups = append(groups, group)
		}
		if r.NextPageToken == "" {
			break
		}
		pageToken = r.NextPageToken
	}
	return groups, nil
}

func fetchGroupMembers(service *admin.Service, group *admin.Group) ([]*admin.Member, error) {
	members := []*admin.Member{}
	pageToken := ""
	for {
		req := service.Members.List(group.Id)
		if pageToken != "" {
			req.PageToken(pageToken)
		}
		r, err := req.Do()
		if err != nil {
			return nil, err
		}
		for _, member := range r.Members {
			members = append(members, member)
		}
		if r.NextPageToken == "" {
			break
		}
		pageToken = r.NextPageToken
	}
	return members, nil
}
