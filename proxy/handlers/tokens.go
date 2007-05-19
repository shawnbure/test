package handlers

import (
	"net/http"
	"strconv"

	"github.com/erdsea/erdsea-api/data/dtos"
	"github.com/erdsea/erdsea-api/services"
	"github.com/erdsea/erdsea-api/storage"
	"github.com/gin-gonic/gin"
)

const (
	baseTokensEndpoint               = "/tokens"
	tokenByTokenIdAndNonceEndpoint   = "/:tokenId/:nonce"
	availableTokensEndpoint          = "/available"
	offersForTokenIdAndNonceEndpoint = "/:tokenId/:nonce/offers/:offset/:limit"
	bidsForTokenIdAndNonceEndpoint   = "/:tokenId/:nonce/bids/:offset/:limit"
	tokenMetadataRelayEndpoint       = "/token/metadata/relay"
)

type tokensHandler struct {
}

type metadataRelayRequest struct {
	Url string `json:"url"`
}

func NewTokensHandler(groupHandler *groupHandler) {
	handler := &tokensHandler{}

	endpoints := []EndpointHandler{
		{Method: http.MethodGet, Path: tokenByTokenIdAndNonceEndpoint, HandlerFunc: handler.getByTokenIdAndNonce},
		{Method: http.MethodPost, Path: availableTokensEndpoint, HandlerFunc: handler.getAvailableTokens},
		{Method: http.MethodGet, Path: offersForTokenIdAndNonceEndpoint, HandlerFunc: handler.getOffers},
		{Method: http.MethodGet, Path: bidsForTokenIdAndNonceEndpoint, HandlerFunc: handler.getBids},
		{Method: http.MethodPost, Path: tokenMetadataRelayEndpoint, HandlerFunc: handler.relayMetadataResponse},
	}

	endpointGroupHandler := EndpointGroupHandler{
		Root:             baseTokensEndpoint,
		Middlewares:      []gin.HandlerFunc{},
		EndpointHandlers: endpoints,
	}

	groupHandler.AddEndpointGroupHandler(endpointGroupHandler)
}

// TODO: authorize the shit out of it
func (handler *tokensHandler) relayMetadataResponse(c *gin.Context) {
	var req metadataRelayRequest

	err := c.Bind(&req)
	if err != nil {
		dtos.JsonResponse(c, http.StatusBadRequest, nil, err.Error())
		return
	}

	var metadataJson map[string]interface{}
	err = services.HttpGet(req.Url, &metadataJson)
	if err != nil {
		dtos.JsonResponse(c, http.StatusNotFound, nil, err.Error())
		return
	}

	dtos.JsonResponse(c, http.StatusOK, metadataJson, "")
}

// @Summary Get token by id and nonce
// @Description Retrieves a token by tokenId and nonce
// @Tags tokens
// @Accept json
// @Produce json
// @Param tokenId path string true "token id"
// @Param nonce path int true "token nonce"
// @Success 200 {object} dtos.ExtendedTokenDto
// @Failure 400 {object} dtos.ApiResponse
// @Failure 404 {object} dtos.ApiResponse
// @Router /tokens/{tokenId}/{nonce} [get]
func (handler *tokensHandler) getByTokenIdAndNonce(c *gin.Context) {
	tokenId := c.Param("tokenId")
	nonceString := c.Param("nonce")

	nonce, err := strconv.ParseUint(nonceString, 10, 64)
	if err != nil {
		dtos.JsonResponse(c, http.StatusBadRequest, nil, err.Error())
		return
	}

	tokenDto, err := services.GetExtendedTokenData(tokenId, nonce)
	if err != nil {
		dtos.JsonResponse(c, http.StatusNotFound, nil, err.Error())
		return
	}

	dtos.JsonResponse(c, http.StatusOK, tokenDto, "")
}

// @Summary Get available tokens
// @Description Get available tokens and some collection info
// @Tags tokens
// @Accept json
// @Produce json
// @Param availableTokensRequest body services.AvailableTokensRequest true "request"
// @Success 200 {object} services.AvailableTokensResponse
// @Failure 400 {object} dtos.ApiResponse
// @Router /tokens/available [get]
func (handler *tokensHandler) getAvailableTokens(c *gin.Context) {
	var request services.AvailableTokensRequest

	err := c.Bind(&request)
	if err != nil {
		dtos.JsonResponse(c, http.StatusBadRequest, nil, err.Error())
		return
	}

	response, err := services.GetAvailableTokens(request)
	if err != nil {
		dtos.JsonResponse(c, http.StatusBadRequest, nil, err.Error())
		return
	}

	dtos.JsonResponse(c, http.StatusOK, response, "")
}

// @Summary Get offers for token
// @Description Retrieves offers for a token (identified by tokenId and nonce)
// @Tags tokens
// @Accept json
// @Produce json
// @Param tokenId path string true "token id"
// @Param nonce path int true "token nonce"
// @Param offset path uint true "offset"
// @Param limit path uint true "limit"
// @Success 200 {object} []dtos.OfferDto
// @Failure 400 {object} dtos.ApiResponse
// @Failure 404 {object} dtos.ApiResponse
// @Router /tokens/{tokenId}/{nonce}/offers/{offset}/{limit} [get]
func (handler *tokensHandler) getOffers(c *gin.Context) {
	tokenId := c.Param("tokenId")
	nonceString := c.Param("nonce")
	offsetStr := c.Param("offset")
	limitStr := c.Param("limit")

	nonce, err := strconv.ParseUint(nonceString, 10, 64)
	if err != nil {
		dtos.JsonResponse(c, http.StatusBadRequest, nil, err.Error())
		return
	}

	offset, err := strconv.ParseUint(offsetStr, 10, 0)
	if err != nil {
		dtos.JsonResponse(c, http.StatusBadRequest, nil, err.Error())
		return
	}

	limit, err := strconv.ParseUint(limitStr, 10, 0)
	if err != nil {
		dtos.JsonResponse(c, http.StatusBadRequest, nil, err.Error())
		return
	}

	err = ValidateLimit(limit)
	if err != nil {
		dtos.JsonResponse(c, http.StatusBadRequest, nil, err.Error())
		return
	}

	tokenCacheInfo, err := services.GetOrAddTokenCacheInfo(tokenId, nonce)
	if err != nil {
		dtos.JsonResponse(c, http.StatusNotFound, nil, err.Error())
		return
	}

	offers, err := storage.GetOffersForTokenWithOffsetLimit(tokenCacheInfo.TokenDbId, int(offset), int(limit))
	if err != nil {
		dtos.JsonResponse(c, http.StatusNotFound, nil, err.Error())
		return
	}

	offersDtos := services.MakeOfferDtos(offers)
	dtos.JsonResponse(c, http.StatusOK, offersDtos, "")
}

// @Summary Get bids for token
// @Description Retrieves bids for a token (identified by tokenId and nonce)
// @Tags tokens
// @Accept json
// @Produce json
// @Param tokenId path string true "token id"
// @Param nonce path int true "token nonce"
// @Param offset path uint true "offset"
// @Param limit path uint true "limit"
// @Success 200 {object} []dtos.BidDto
// @Failure 400 {object} dtos.ApiResponse
// @Failure 404 {object} dtos.ApiResponse
// @Router /tokens/{tokenId}/{nonce}/bids/{offset}/{limit} [get]
func (handler *tokensHandler) getBids(c *gin.Context) {
	tokenId := c.Param("tokenId")
	nonceString := c.Param("nonce")
	offsetStr := c.Param("offset")
	limitStr := c.Param("limit")

	nonce, err := strconv.ParseUint(nonceString, 10, 64)
	if err != nil {
		dtos.JsonResponse(c, http.StatusBadRequest, nil, err.Error())
		return
	}

	offset, err := strconv.ParseUint(offsetStr, 10, 0)
	if err != nil {
		dtos.JsonResponse(c, http.StatusBadRequest, nil, err.Error())
		return
	}

	limit, err := strconv.ParseUint(limitStr, 10, 0)
	if err != nil {
		dtos.JsonResponse(c, http.StatusBadRequest, nil, err.Error())
		return
	}

	err = ValidateLimit(limit)
	if err != nil {
		dtos.JsonResponse(c, http.StatusBadRequest, nil, err.Error())
		return
	}

	tokenCacheInfo, err := services.GetOrAddTokenCacheInfo(tokenId, nonce)
	if err != nil {
		dtos.JsonResponse(c, http.StatusNotFound, nil, err.Error())
		return
	}

	bids, err := storage.GetBidsForTokenWithOffsetLimit(tokenCacheInfo.TokenDbId, int(offset), int(limit))
	if err != nil {
		dtos.JsonResponse(c, http.StatusNotFound, nil, err.Error())
		return
	}

	bidsDtos := services.MakeBidDtos(bids)
	dtos.JsonResponse(c, http.StatusOK, bidsDtos, "")
}
