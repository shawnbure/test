package handlers

import (
	"net/http"

	"github.com/erdsea/erdsea-api/config"
	"github.com/erdsea/erdsea-api/data"
	"github.com/erdsea/erdsea-api/proxy/middleware"
	"github.com/erdsea/erdsea-api/services"
	"github.com/gin-gonic/gin"
)

const (
	baseSearchEndpoint        = "/search"
	generalSearchEndpoint     = "/:searchString"
	collectionsSearchEndpoint = "/collections/:collectionName"
	accountsSearchEndpoint    = "/accounts/:accountName"

	SearchCategoryLimit = 5
)

type GeneralSearchResponse struct {
	Accounts    []data.Account
	Collections []data.Collection
}

type searchHandler struct {
}

func NewSearchHandler(groupHandler *groupHandler, authCfg config.AuthConfig) {
	handler := &searchHandler{}

	endpoints := []EndpointHandler{
		{Method: http.MethodGet, Path: generalSearchEndpoint, HandlerFunc: handler.search},
		{Method: http.MethodGet, Path: collectionsSearchEndpoint, HandlerFunc: handler.collectionSearch},
		{Method: http.MethodGet, Path: accountsSearchEndpoint, HandlerFunc: handler.accountSearch},
	}

	endpointGroupHandler := EndpointGroupHandler{
		Root:             baseSearchEndpoint,
		Middlewares:      []gin.HandlerFunc{middleware.Authorization(authCfg.JwtSecret)},
		EndpointHandlers: endpoints,
	}

	groupHandler.AddEndpointGroupHandler(endpointGroupHandler)
}

// @Summary General search by string.
// @Description Searches for collections by name and accounts by name. Cached for 20 minutes. Limit 5 elements for each.
// @Tags search
// @Accept json
// @Produce json
// @Param searchString path string true "search string"
// @Success 200 {object} GeneralSearchResponse
// @Failure 500 {object} data.ApiResponse
// @Router /search/{searchString} [get]
func (handler *searchHandler) search(c *gin.Context) {
	searchString := c.Param("searchString")

	collections, err := services.GetCollectionsWithNameAlike(searchString, SearchCategoryLimit)
	if err != nil {
		data.JsonResponse(c, http.StatusInternalServerError, nil, err.Error())
		return
	}

	accounts, err := services.GetAccountsWithNameAlike(searchString, SearchCategoryLimit)
	if err != nil {
		data.JsonResponse(c, http.StatusInternalServerError, nil, err.Error())
		return
	}

	response := GeneralSearchResponse{
		Accounts:    accounts,
		Collections: collections,
	}
	data.JsonResponse(c, http.StatusOK, response, "")
}

// @Summary Search collections by name.
// @Description Searches for collections by name. Cached for 20 minutes. Limit 5 elements.
// @Tags search
// @Accept json
// @Produce json
// @Param collectionName path string true "search string"
// @Success 200 {object} []data.Collection
// @Failure 500 {object} data.ApiResponse
// @Router /search/collections/{collectionName} [get]
func (handler *searchHandler) collectionSearch(c *gin.Context) {
	collectionName := c.Param("collectionName")

	collections, err := services.GetCollectionsWithNameAlike(collectionName, SearchCategoryLimit)
	if err != nil {
		data.JsonResponse(c, http.StatusInternalServerError, nil, err.Error())
		return
	}

	data.JsonResponse(c, http.StatusOK, collections, "")
}

// @Summary Search accounts by name.
// @Description Searches for accounts by name. Cached for 20 minutes. Limit 5 elements.
// @Tags search
// @Accept json
// @Produce json
// @Param accountName path string true "search string"
// @Success 200 {object} []data.Account
// @Failure 500 {object} data.ApiResponse
// @Router /search/accounts/{accountName} [get]
func (handler *searchHandler) accountSearch(c *gin.Context) {
	accountName := c.Param("accountName")

	accounts, err := services.GetAccountsWithNameAlike(accountName, SearchCategoryLimit)
	if err != nil {
		data.JsonResponse(c, http.StatusInternalServerError, nil, err.Error())
		return
	}

	data.JsonResponse(c, http.StatusOK, accounts, "")
}
