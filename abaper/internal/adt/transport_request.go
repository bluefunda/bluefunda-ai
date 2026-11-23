package adt

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/bluefunda/bluefunda-ai/abaper/types"
	"go.uber.org/zap"
)

// transportOrganizerMediaType is the media type of the ADT transport
// organizer (Eclipse "Transport Organizer" view). The same Accept header is
// used by abap-adt-api (transportDetails) and sapcli (sap/adt/cts.py).
const transportOrganizerMediaType = "application/vnd.sap.adt.transportorganizer.v1+xml"

// transportRequestsEndpoint is relative to baseURL (/sap/bc/adt).
const transportRequestsEndpoint = "/cts/transportrequests/"

// ErrInvalidTransportNumber is returned for a transport number that does not
// have the SAP form <SID>K<6 digits>, e.g. "DEVK900123".
var ErrInvalidTransportNumber = errors.New("invalid transport number")

// transportNumberRe is the SAP request/task number (TRKORR) convention: a
// 3-character system ID (starting with a letter), "K", then a 6-digit
// sequence number.
var transportNumberRe = regexp.MustCompile(`^[A-Z][A-Z0-9]{2}K[0-9]{6}$`)

// NormalizeTransportNumber trims and upper-cases trkey and checks it is a
// valid SAP transport number.
func NormalizeTransportNumber(trkey string) (string, error) {
	n := strings.ToUpper(strings.TrimSpace(trkey))
	if !transportNumberRe.MatchString(n) {
		return "", fmt.Errorf("%w: %q (expected <SID>K<6 digits>, e.g. DEVK900123)", ErrInvalidTransportNumber, trkey)
	}
	return n, nil
}

// Transport organizer XML (namespace http://www.sap.com/cts/adt/tm). Only
// local names are matched, so the "tm:" prefix is irrelevant to the decoder.
//
//	<tm:root>
//	  <tm:request tm:number tm:owner tm:desc tm:status tm:type tm:target ...>
//	    <tm:abap_object .../>            (objects of released tasks)
//	    <tm:task tm:number tm:owner tm:desc tm:status ...>
//	      <tm:abap_object tm:pgmid tm:type tm:name tm:obj_desc tm:obj_info .../>
//	    </tm:task>
//	  </tm:request>
//	</tm:root>
type tmRootXML struct {
	Request *tmRequestXML `xml:"request"`
}

type tmRequestXML struct {
	Number      string        `xml:"number,attr"`
	Owner       string        `xml:"owner,attr"`
	Description string        `xml:"desc,attr"`
	Status      string        `xml:"status,attr"`
	Type        string        `xml:"type,attr"`
	Target      string        `xml:"target,attr"`
	Tasks       []tmTaskXML   `xml:"task"`
	Objects     []tmObjectXML `xml:"abap_object"`
}

type tmTaskXML struct {
	Number      string        `xml:"number,attr"`
	Owner       string        `xml:"owner,attr"`
	Description string        `xml:"desc,attr"`
	Status      string        `xml:"status,attr"`
	Objects     []tmObjectXML `xml:"abap_object"`
}

type tmObjectXML struct {
	PgmID       string `xml:"pgmid,attr"`
	Type        string `xml:"type,attr"`
	Name        string `xml:"name,attr"`
	Description string `xml:"obj_desc,attr"`
	Info        string `xml:"obj_info,attr"` // object type text, e.g. "Class"; fallback description
}

// GetTransportRequest reads a transport request, its tasks and the objects
// recorded in them, via GET /sap/bc/adt/cts/transportrequests/{trkey}.
// It is read-only. A request that does not exist returns ErrNotFound.
func (c *ADTClientImpl) GetTransportRequest(ctx context.Context, trkey string) (*types.ADTTransportRequest, error) {
	if !c.IsAuthenticated() {
		return nil, fmt.Errorf("client not authenticated - call Authenticate() first")
	}
	number, err := NormalizeTransportNumber(trkey)
	if err != nil {
		return nil, err
	}

	reqURL := fmt.Sprintf("%s%s%s?sap-client=%s&sap-language=%s",
		c.baseURL, transportRequestsEndpoint, url.PathEscape(number),
		url.QueryEscape(c.config.Client), url.QueryEscape(c.config.Language))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	c.addAuthHeaders(req)
	req.Header.Set("Accept", transportOrganizerMediaType)

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%w: transport request %s", ErrNotFound, number)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get transport request %s failed: HTTP %d - %s", number, resp.StatusCode, truncateForError(body))
	}

	result, err := parseTransportRequestXML(body)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("%w: transport request %s", ErrNotFound, number)
	}

	c.logger.Info("Transport request retrieved",
		zap.String("trkey", number),
		zap.Int("tasks", len(result.Tasks)))
	return result, nil
}

// parseTransportRequestXML converts a transport organizer response into an
// ADTTransportRequest. It returns (nil, nil) when the response holds no
// request.
func parseTransportRequestXML(body []byte) (*types.ADTTransportRequest, error) {
	var root tmRootXML
	if err := xml.Unmarshal(body, &root); err != nil {
		return nil, fmt.Errorf("failed to parse transport request response: %w", err)
	}
	if root.Request == nil || root.Request.Number == "" {
		return nil, nil
	}
	r := root.Request
	out := &types.ADTTransportRequest{
		Number:      r.Number,
		Description: r.Description,
		Owner:       r.Owner,
		Status:      r.Status,
		Type:        r.Type,
		Target:      r.Target,
		Tasks:       make([]types.ADTTransportTask, 0, len(r.Tasks)),
		Objects:     convertTransportObjects(r.Objects),
	}
	for _, t := range r.Tasks {
		out.Tasks = append(out.Tasks, types.ADTTransportTask{
			Number:      t.Number,
			Owner:       t.Owner,
			Description: t.Description,
			Status:      t.Status,
			Objects:     convertTransportObjects(t.Objects),
		})
	}
	if len(out.Objects) == 0 {
		out.Objects = nil
	}
	return out, nil
}

func convertTransportObjects(in []tmObjectXML) []types.ADTTransportObject {
	out := make([]types.ADTTransportObject, 0, len(in))
	for _, o := range in {
		desc := o.Description
		if desc == "" {
			desc = o.Info
		}
		out = append(out, types.ADTTransportObject{
			PgmID:       o.PgmID,
			Type:        o.Type,
			Name:        o.Name,
			Description: desc,
		})
	}
	return out
}

// truncateForError keeps SAP error bodies short in returned errors.
func truncateForError(body []byte) string {
	const maxLen = 512
	s := strings.TrimSpace(string(body))
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
