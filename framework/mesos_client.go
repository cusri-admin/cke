package framework

import (
	"bytes"
	"cke/utils"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/golang/protobuf/proto"
	mesos "github.com/mesos/mesos-go/api/v1/lib"
	mm "github.com/mesos/mesos-go/api/v1/lib/master"
)

func callMesosHTTP(server string, reqData []byte, readLength int64) ([]byte, error) {
	req, err := http.NewRequest("POST", "http://"+server+"/api/v1", bytes.NewReader(reqData))

	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/x-protobuf")
	//req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Type", "application/x-protobuf")

	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	defer func() {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
	}()
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		buf, err := ioutil.ReadAll(io.LimitReader(resp.Body, 16*1024))
		if err != nil {
			return nil, utils.NewHttpError(http.StatusInternalServerError, "Error read response")
		}
		return nil, utils.NewHttpError(resp.StatusCode, string(buf)) //errors.New("Http response status code Error: " + strconv.Itoa(resp.StatusCode) + " body:" + string(buf))
	}

	//body, err := ioutil.ReadAll(resp.Body)
	body, err := ioutil.ReadAll(io.LimitReader(resp.Body, readLength))
	if err != nil {
		return nil, utils.NewHttpError(http.StatusInternalServerError, "Error read response")
	}
	return body, nil
}

func callOperatorUnreserve(cpus, mem float64, hostname, agentID, role, key, value string) error {
	call := &mm.Call{
		Type: mm.Call_UNRESERVE_RESOURCES,
		UnreserveResources: &mm.Call_UnreserveResources{
			AgentID: mesos.AgentID{
				Value: agentID,
			},
			Resources: []mesos.Resource{
				mesos.Resource{
					Name:   "cpus",
					Type:   mesos.SCALAR.Enum(),
					Scalar: &mesos.Value_Scalar{Value: cpus},
					Role:   &role,
					Reservation: &mesos.Resource_ReservationInfo{
						Labels: &mesos.Labels{
							Labels: []mesos.Label{
								mesos.Label{
									Key:   key,
									Value: &value,
								},
							},
						},
					},
				},
				mesos.Resource{
					Name:   "mem",
					Type:   mesos.SCALAR.Enum(),
					Scalar: &mesos.Value_Scalar{Value: mem},
					Role:   &role,
					Reservation: &mesos.Resource_ReservationInfo{
						Labels: &mesos.Labels{
							Labels: []mesos.Label{
								mesos.Label{
									Key:   key,
									Value: &value,
								},
							},
						},
					},
				},
			},
		},
	}

	data, err := proto.Marshal(call)
	if err != nil {
		return err
	}

	respData, err := callMesosHTTP(hostname, data, 1024)

	if err != nil {
		return fmt.Errorf("(%s)call %s unreservation error: %s", hostname, agentID, err.Error())
	}

	resp := &mm.Response{}
	//解码数据
	if err := proto.Unmarshal(respData, resp); err != nil {
		return fmt.Errorf("(%s)call %s unreservation error: %s", hostname, agentID, err.Error())
	}
	return nil
}
