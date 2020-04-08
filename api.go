package lraft

import (
	"fmt"
	"net/http"

	"github.com/emicklei/go-restful"
	restfulspec "github.com/emicklei/go-restful-openapi"
	"go.mongodb.org/mongo-driver/mongo"
)

type StoreAdd interface {
	// Get returns the value for the given key.
	Get(key string) (string, error)

	// Set sets the value for the given key, via distributed consensus.
	Set(key, value string) error

	// Delete removes the given key, via distributed consensus.
	Delete(key string) error

	// Join joins the node, identitifed by nodeID and reachable at addr, to the cluster.
	Join(nodeID string, addr string) error
}

type LeveldbResource struct {
	// normally one would use DAO (data access object)
	Store StoreAdd
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
func (lr *LeveldbResource) WebService() *restful.WebService {
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
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Reads(KV{}))

	ws.Route(ws.DELETE("/{key}").To(lr.delete).
		Doc("删除key").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Returns(http.StatusOK, "OK", mongo.DeleteResult{}).
		Returns(http.StatusInternalServerError, "server error", nil))

	return ws
}

func (lr *LeveldbResource) get(request *restful.Request, response *restful.Response) {
	key := request.PathParameter("key")
	v, err := lr.Store.Get(key)
	if err != nil {
		response.WriteError(http.StatusInternalServerError, nil)
		return
	}
	response.WriteEntity(&Msg{Data: v, Code: http.StatusOK, Message: http.StatusText(http.StatusOK)})
}

func (lr *LeveldbResource) put(request *restful.Request, response *restful.Response) {
	var kv KV
	if err := request.ReadEntity(&kv); err != nil {
		response.WriteError(http.StatusInternalServerError, err)
	}
	err := lr.Store.Set(kv.Key, kv.Value)
	if err != nil {
		response.WriteError(http.StatusInternalServerError, err)
		return
	}
	response.WriteEntity(&Msg{Code: http.StatusOK, Data: fmt.Sprintf("%v-%v", kv.Key, kv.Value), Message: http.StatusText(http.StatusOK)})
}

func (lr *LeveldbResource) delete(request *restful.Request, response *restful.Response) {
	key := request.PathParameter("key")
	err := lr.Store.Delete(key)
	if err != nil {
		response.WriteError(http.StatusInternalServerError, err)
		return
	}
	response.WriteEntity(&Msg{Code: http.StatusOK, Message: http.StatusText(http.StatusOK)})
}
