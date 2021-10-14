package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type OntapClient struct {
	username      string
	password      string
	URL           url.URL
	httpClient    http.Client
	trustedSSHKey string
}

type OntapErrBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Target  string `json:"target"`
}

type OntapErrResponse struct {
	Error OntapErrBody `json:"error"`
}

type OntapError struct {
	err        string
	body       OntapErrResponse
	statusCode int
}

func (e OntapError) Error() string {
	return fmt.Sprintf("Status code: %v, Ontap error code: %v, Message: %s", e.statusCode, e.body.Error.Code, e.body.Error.Message)
}

type OntapResponse struct {
	body       []byte
	statusCode int
}

func NewOntapClient(username, password, trustedSSHKey, urlString string, skipssl bool) (*OntapClient, error) {
	urlParsed, err := url.Parse(urlString)
	if err != nil {
		return nil, fmt.Errorf("Error parsing storageGrid URL: %v", err.Error())
	}

	if urlParsed.Path == "" {
		urlParsed.Path = "/api"
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: skipssl},
	}

	httpClient := http.Client{Transport: tr}

	return &OntapClient{
		username:      username,
		password:      password,
		URL:           *urlParsed,
		httpClient:    httpClient,
		trustedSSHKey: trustedSSHKey,
	}, nil
}

func (o *OntapClient) DoApiRequest(method, path string, body []byte, checkForCode int) (OntapResponse, error) {
	var apiResp OntapResponse

	req, err := http.NewRequest(method, fmt.Sprintf("%s/%s", o.URL.String(), path), bytes.NewReader(body))
	if err != nil {
		return OntapResponse{}, fmt.Errorf("Error creating request: %s", err)
	}

	req.Header.Add("Content-Type", "application/json")
	req.SetBasicAuth(o.username, o.password)

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return apiResp, fmt.Errorf("Error doing http request: %s", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != checkForCode {
		var eRes OntapErrResponse
		json.NewDecoder(resp.Body).Decode(&eRes)
		return apiResp, OntapError{
			err:        eRes.Error.Message,
			body:       eRes,
			statusCode: resp.StatusCode,
		}
	}

	bdy, _ := ioutil.ReadAll(resp.Body)
	apiResp.statusCode = resp.StatusCode
	apiResp.body = bdy

	return apiResp, nil
}

func (o *OntapClient) GetVolumeIDByName(name string) (string, error) {
	res, err := o.DoApiRequest(http.MethodGet, fmt.Sprintf("/storage/volumes?name=%s", name), nil, 200)
	if err != nil {
		return "", err
	}

	var list ResultList
	err = json.Unmarshal(res.body, &list)
	if err != nil {
		return "", fmt.Errorf("Unable to parse result..")
	}

	if list.NumRecords != 1 {
		return "", fmt.Errorf("Didn't find the expected (1) number of records for volumes with name %s", name)
	}

	return list.Records[0].UUID, nil
}

func (o *OntapClient) GetSvmIdByName(name string) (string, error) {
	res, err := o.DoApiRequest(http.MethodGet, fmt.Sprintf("/svm/svms?name=%s", name), nil, 200)
	if err != nil {
		return "", err
	}

	var list ResultList
	err = json.Unmarshal(res.body, &list)
	if err != nil {
		return "", fmt.Errorf("Unable to parse result..")
	}

	if list.NumRecords != 1 {
		return "", fmt.Errorf("Didn't find the expected (1) number of records for volumes with name %s", name)
	}

	return list.Records[0].UUID, nil
}

func (o *OntapClient) CreateVolume(name, svmName, aggName, comment, exportPolicy string, size int64) (string, error) {
	v := Volume{}
	v.Name = name
	v.Comment = comment
	v.Size = size
	v.Svm.Name = svmName
	v.Aggregates = append(v.Aggregates, Aggregate{Name: aggName})
	v.Nas.ExportPolicy.Name = exportPolicy

	bdy, _ := json.Marshal(v)
	res, err := o.DoApiRequest(http.MethodPost, "/storage/volumes", bdy, 202)
	if err != nil {
		return "", err
	}

	var ar AcceptResponse
	err = json.Unmarshal(res.body, &ar)
	if err != nil {
		return "", fmt.Errorf("Did not get expected response body. Got instead: %s", string(res.body))
	}

	return ar.Job.UUID, nil
}

func (o *OntapClient) CreateCifsVolume(name, svmName string, size int64) (string, error) {
	v := CifsApplication{}
	v.Name = name
	v.SmartContainer = true
	v.Svm.Name = svmName
	v.Template.Name = "nas"
	v.Nas.NfsAccess = []interface{}{}
	v.Nas.CifsAccess = append(v.Nas.CifsAccess, CifsAccess{
		Access:      "No_access",
		UserOrGroup: "BUILTIN\\Guests",
	})
	v.Nas.ProtectionType.LocalPolicy = "none"
	v.Nas.ProtectionType.RemoteRpo = "none"
	v.Nas.ApplicationComponents = append(v.Nas.ApplicationComponents, ApplicationComponents{
		Name:       name,
		TotalSize:  size,
		ShareCount: 1,
		ScaleOut:   false,
		Tiering: struct {
			Control string "json:\"control\""
		}{Control: "disallowed"},
		StorageService: struct {
			Name string "json:\"name\""
		}{
			Name: "value",
		},
	})

	bdy, _ := json.Marshal(v)
	res, err := o.DoApiRequest(http.MethodPost, "/application/applications", bdy, 202)
	if err != nil {
		return "", err
	}

	var ar AcceptResponse
	err = json.Unmarshal(res.body, &ar)
	if err != nil {
		return "", fmt.Errorf("Did not get expected response body. Got instead: %s", string(res.body))
	}

	return ar.Job.UUID, nil
}

func (o *OntapClient) GetJobStatus(uuid string) (OntapResponse, error) {
	return o.DoApiRequest(http.MethodGet, fmt.Sprintf("/cluster/jobs/%s", uuid), nil, 200)
}

func (o *OntapClient) DeleteVolume(uuid string) (string, error) {
	res, err := o.DoApiRequest(http.MethodDelete, fmt.Sprintf("/storage/volumes/%s", uuid), nil, 202)
	if err != nil {
		return "", err
	}

	var ar AcceptResponse
	err = json.Unmarshal(res.body, &ar)
	if err != nil {
		return "", fmt.Errorf("Did not get expected response body. Got instead: %s", string(res.body))
	}

	return ar.Job.UUID, nil
}

func (o *OntapClient) GetVolumeByID(uuid string) (Volume, error) {
	res, err := o.DoApiRequest(http.MethodGet, fmt.Sprintf("/storage/volumes/%s?fields=nas.path", uuid), nil, 200)
	if err != nil {
		return Volume{}, err
	}

	var vol Volume
	err = json.Unmarshal(res.body, &vol)
	if err != nil {
		return Volume{}, fmt.Errorf("Error pasring volume response json")
	}

	return vol, nil
}

func (o *OntapClient) StartSSHSession() (*ssh.Client, *ssh.Session, error) {
	sshConfig := &ssh.ClientConfig{
		User: o.username,
		Auth: []ssh.AuthMethod{
			ssh.Password(o.password),
		},
		HostKeyCallback: trustedHostKeyCallback(o.trustedSSHKey),
		Timeout:         15 * time.Second,
	}

	connection, err := ssh.Dial("tcp", fmt.Sprintf("%s:%s", o.URL.Hostname(), "22"), sshConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to dial: %s", err)
	}

	session, err := connection.NewSession()
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to create session: %s", err)
	}

	return connection, session, nil
}

func (o *OntapClient) CreateCifsUser(username, password, fullName string) error {
	connection, session, err := o.StartSSHSession()
	if err != nil {
		return err
	}
	defer connection.Close()
	defer session.Close()

	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("%s", err.Error())
	}

	var b bytes.Buffer
	session.Stdout = &b
	session.Stderr = &b

	cmd := fmt.Sprintf("vserver cifs users-and-groups local-user create -user-name %s -full-name %s", username, fullName)
	session.Start(cmd)
	time.Sleep(250 * time.Millisecond)
	fmt.Fprintf(stdin, "%s\n", password)
	time.Sleep(250 * time.Millisecond)
	fmt.Fprintf(stdin, "%s\n", password)
	time.Sleep(10 * time.Millisecond)
	fmt.Fprintf(stdin, "%s\n", "exit")
	session.Wait()

	return nil
}

func (o *OntapClient) GetCifsUserByFullname(svmName, fullName string) (string, error) {
	var username string

	connection, session, err := o.StartSSHSession()
	if err != nil {
		return "", err
	}
	defer connection.Close()
	defer session.Close()

	cmd := fmt.Sprintf("vserver cifs users-and-groups local-user show -fields user-name -full-name %s", fullName)
	out, err := session.CombinedOutput(cmd)
	if err != nil {
		return "", err
	}

	for _, line := range strings.Split(strings.TrimSuffix(string(out), "\n"), "\n") {
		if strings.HasPrefix(line, svmName) {
			username = strings.TrimSpace(strings.TrimLeft(line, svmName))
		}
	}

	return username, nil
}

func (o *OntapClient) DeleteCifsUser(username string) error {
	connection, session, err := o.StartSSHSession()
	if err != nil {
		return err
	}
	defer connection.Close()
	defer session.Close()

	cmd := fmt.Sprintf("vserver cifs users-and-groups local-user delete -user-name %s", username)
	err = session.Run(cmd)
	if err != nil {
		return err
	}

	return nil
}

func (o *OntapClient) AssignCifsUser(username, svmId, shareName string) error {
	acl := cifsACL{
		UserOrGroup: username,
		Type:        "windows",
		Permission:  "full_control",
	}

	bdy, _ := json.Marshal(acl)
	_, err := o.DoApiRequest(http.MethodPost, fmt.Sprintf("/protocols/cifs/shares/%s/%s/acls", svmId, shareName), bdy, 201)
	if err != nil {
		return err
	}

	return nil
}
