package handlers

import (
	"fmt"
	"github.com/ENFT-DAO/youbei-api/cache"
	"github.com/ENFT-DAO/youbei-api/data/dtos"
	"github.com/ENFT-DAO/youbei-api/data/entities"
	"github.com/ENFT-DAO/youbei-api/storage"
	logger "github.com/ElrondNetwork/elrond-go-logger"
	"github.com/gin-gonic/gin"
	"math/big"
	"net/http"
	"strings"
	"time"
)

var (
	StatsTotalVolumeKeyFormat           = "Stats:Volume:Total"
	StatsTotalVolumeLastUpdateKeyFormat = "Stats:Volume:TotalLastUpdate"
	StatsTotalVolumeExpirePeriod        = 2 * time.Hour

	StatsTotalVolumePerDayKeyFormat    = "Stats:Volume:%s"
	StatsTotalVolumePerDayExpirePeriod = time.Hour * 24 * 1
)

var (
	logInstance = logger.GetOrCreate("stats-handler")
)

const (
	baseStatsEndpoint                   = "/stats"
	StatTransactionsCountEndpoint       = "/txCount"
	StatTransactionsCountByDateEndpoint = "/txCount/:date"
	StatTotalVolumeEndpoint             = "/volume/total"
	StatTotalVolumeLastWeekPerDay       = "/volume/last_week"
)

type statsHandler struct {
}

func NewStatsHandler(groupHandler *groupHandler) {
	handler := &statsHandler{}

	endpoints := []EndpointHandler{
		{Method: http.MethodGet, Path: StatTransactionsCountEndpoint, HandlerFunc: handler.getTradeCounts},
		{Method: http.MethodGet, Path: StatTransactionsCountByDateEndpoint, HandlerFunc: handler.getTradeCounts},
		{Method: http.MethodGet, Path: StatTotalVolumeEndpoint, HandlerFunc: handler.getTotalTradesVolume},
		{Method: http.MethodGet, Path: StatTotalVolumeLastWeekPerDay, HandlerFunc: handler.getTotalTradesVolumeLastWeek},
	}

	endpointGroupHandler := EndpointGroupHandler{
		Root:             baseStatsEndpoint,
		Middlewares:      []gin.HandlerFunc{},
		EndpointHandlers: endpoints,
	}

	groupHandler.AddEndpointGroupHandler(endpointGroupHandler)
}

// @Summary Gets transactions count.
// @Description Gets transactions count (total/buy/withdraw/...) and can be filtered by date
// @Tags transactions
// @Accept json
// @Produce json
// @Success 200 {object} dtos.TradesCount
// @Failure 400 {object} dtos.ApiResponse
// @Failure 404 {object} dtos.ApiResponse
// @Router /stats/txCount/{date} [get]
func (handler *statsHandler) getTradeCounts(c *gin.Context) {
	filterDate := c.Param("date")

	result := dtos.TradesCount{}
	if strings.TrimSpace(filterDate) == "" {
		totalCount, err := storage.GetTransactionsCount()
		if err != nil {
			dtos.JsonResponse(c, http.StatusInternalServerError, nil, err.Error())
			return
		}
		result.Total = totalCount

		totalBuy, err := storage.GetTransactionsCountByType(entities.BuyToken)
		if err != nil {
			dtos.JsonResponse(c, http.StatusInternalServerError, nil, err.Error())
			return
		}
		result.Buy = totalBuy

		totalWithdraw, err := storage.GetTransactionsCountByType(entities.WithdrawToken)
		if err != nil {
			dtos.JsonResponse(c, http.StatusInternalServerError, nil, err.Error())
			return
		}
		result.Withdraw = totalWithdraw
	} else {
		totalCount, err := storage.GetTransactionsCountByDate(filterDate)
		if err != nil {
			dtos.JsonResponse(c, http.StatusInternalServerError, nil, err.Error())
			return
		}
		result.Total = totalCount

		totalBuy, err := storage.GetTransactionsCountByDateAndType(entities.BuyToken, filterDate)
		if err != nil {
			dtos.JsonResponse(c, http.StatusInternalServerError, nil, err.Error())
			return
		}
		result.Buy = totalBuy

		totalWithdraw, err := storage.GetTransactionsCountByDateAndType(entities.WithdrawToken, filterDate)
		if err != nil {
			dtos.JsonResponse(c, http.StatusInternalServerError, nil, err.Error())
			return
		}
		result.Withdraw = totalWithdraw

	}

	dtos.JsonResponse(c, http.StatusOK, result, "")
}

// @Summary Gets Total Volume
// @Description Gets Total Volume
// @Tags transactions
// @Accept json
// @Produce json
// @Success 200 {object} dtos.TradesVolumeTotal
// @Failure 400 {object} dtos.ApiResponse
// @Failure 404 {object} dtos.ApiResponse
// @Router /stats/volume/total [get]
func (handler *statsHandler) getTotalTradesVolume(c *gin.Context) {
	result := dtos.TradesVolumeTotal{}

	// Let's check the cache first
	localCacher := cache.GetLocalCacher()

	var totalVolume *big.Float
	var totalVolumeLastUpdate int64

	totalLU, errRead := localCacher.Get(StatsTotalVolumeLastUpdateKeyFormat)
	totalStr, errRead2 := localCacher.Get(StatsTotalVolumeKeyFormat)
	if errRead == nil && errRead2 == nil {
		totalVolume, _ = new(big.Float).SetString(totalStr.(string))
		totalVolumeLastUpdate = totalLU.(int64)
	} else {
		// get it from database and also cache it
		totalV, err := storage.GetTotalTradedVolume()
		if err != nil {
			dtos.JsonResponse(c, http.StatusInternalServerError, nil, err.Error())
			return
		}
		totalVolume = totalV
		totalVolumeLastUpdate = time.Now().UTC().Unix()

		err = localCacher.SetWithTTLSync(StatsTotalVolumeKeyFormat, totalV.String(), StatsTotalVolumeExpirePeriod)
		if err != nil {
			logInstance.Debug("could not set cache", "err", err)
		}

		err = localCacher.SetWithTTLSync(StatsTotalVolumeLastUpdateKeyFormat, totalVolumeLastUpdate, StatsTotalVolumeExpirePeriod)
		if err != nil {
			logInstance.Debug("could not set cache", "err", err)
		}
	}

	result.Sum = totalVolume.String()
	result.LastUpdate = totalVolumeLastUpdate

	dtos.JsonResponse(c, http.StatusOK, result, "")
}

// @Summary Gets Total Volume Per Day For Last Week
// @Description Gets Total Volume Per Day For Last Week
// @Tags transactions
// @Accept json
// @Produce json
// @Success 200 {object} []dtos.TradesVolume
// @Failure 400 {object} dtos.ApiResponse
// @Failure 404 {object} dtos.ApiResponse
// @Router /stats/volume/last_week [get]
func (handler *statsHandler) getTotalTradesVolumeLastWeek(c *gin.Context) {
	result := []dtos.TradesVolume{}

	// Let's find out today
	today := time.Now().UTC()
	//dateFormat := "2021-02-01"

	// Let's check the cache first
	localCacher := cache.GetLocalCacher()

	for i := 1; i < 8; i++ {
		tempDate := today.Add(-24 * time.Duration(i) * time.Hour)
		finalDate := fmt.Sprintf("%4d-%02d-%02d", tempDate.Year(), tempDate.Month(), tempDate.Day())

		var totalVolume *big.Float

		key := fmt.Sprintf(StatsTotalVolumePerDayKeyFormat, finalDate)
		totalStr, errRead := localCacher.Get(key)
		if errRead == nil {
			totalVolume, _ = new(big.Float).SetString(totalStr.(string))
		} else {
			// get it from database and also cache it
			totalV, err := storage.GetTotalTradedVolumeByDate(finalDate)
			if err != nil {
				totalVolume = big.NewFloat(0)
				//dtos.JsonResponse(c, http.StatusInternalServerError, nil, err.Error())
				//return
			}
			totalVolume = totalV

			err = localCacher.SetWithTTLSync(StatsTotalVolumeKeyFormat, totalV.String(), StatsTotalVolumePerDayExpirePeriod)
			if err != nil {
				logInstance.Debug("could not set cache", "err", err)
			}
		}

		result = append(result, dtos.TradesVolume{
			Sum:  totalVolume.String(),
			Date: finalDate,
		})
	}

	dtos.JsonResponse(c, http.StatusOK, result, "")
}
