package lraft

import (
	"fmt"
	"net/http"

	"github.com/emicklei/go-restful"
	restfulspec "github.com/emicklei/go-restful-openapi"
	"github.com/prometheus/alertmanager/template"
	"go.mongodb.org/mongo-driver/mongo"
)

type LevaldbResource struct {
	// normally one would use DAO (data access object)
	store *Store
}

type Msg struct {
	Code    int    `json:"code"`
	Data    string `json:"data"`
	Message string `json"message"`
}

type KV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// WebService creates a new service that can handle REST requests for User resources.
func (lr LevaldbResource) WebService() *restful.WebService {
	ws := new(restful.WebService)
	ws.
		Path("/lraft").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON) // you can specify this per route as well

	tags := []string{"levalDB Raft"}

	ws.Route(ws.GET("/{key}").To(lr.get).
		Doc("根据key获取信息").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Returns(http.StatusOK, "OK", Msg{}).
		Returns(http.StatusBadRequest, "bad request", nil).
		Returns(http.StatusInternalServerError, "server error", nil))

	ws.Route(ws.POST("/put").To(lr.put).
		// docs
		Doc("添加 k-v").
		Writes(KV{}).
		Returns(http.StatusOK, "OK", Msg{}).
		//Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Reads(template.Data{})) // from the request

	ws.Route(ws.DELETE("/{key}").To(lr.delete).
		Doc("删除key").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Returns(http.StatusOK, "OK", mongo.DeleteResult{}).
		Returns(http.StatusInternalServerError, "server error", nil))

	return ws
}

func (lr *LevaldbResource) get(request *restful.Request, response *restful.Response) {
	key := request.PathParameter("key")
	v, err := lr.store.Get(key)
	if err == nil {
		response.WriteEntity(&Msg{Data: v, Code: http.StatusOK, Message: http.StatusText(http.StatusOK)})
	} else {
		response.WriteError(http.StatusInternalServerError, err)
	}
}

func (lr *LevaldbResource) put(request *restful.Request, response *restful.Response) {
	var kv KV
	if err := request.ReadEntity(&kv); err != nil {
		response.WriteError(http.StatusInternalServerError, err)
	}

	err := lr.store.Set(kv.Key, kv.Value)
	if err == nil {
		response.WriteEntity(&Msg{Code: http.StatusOK, Data: fmt.Sprintf("%v-%v", kv.Key, kv.Value), Message: http.StatusText(http.StatusOK)})
	} else {
		response.WriteError(http.StatusInternalServerError, err)
	}
}

func (lr *LevaldbResource) delete(request *restful.Request, response *restful.Response) {
	key := request.PathParameter("key")
	err := lr.store.Delete(key)
	if err == nil {
		response.WriteEntity(&Msg{Code: http.StatusOK, Message: http.StatusText(http.StatusOK)})
	} else {
		response.WriteError(http.StatusInternalServerError, err)
	}
}
