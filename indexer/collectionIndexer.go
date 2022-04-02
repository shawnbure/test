package indexer

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ENFT-DAO/youbei-api/data/dtos"
	"github.com/ENFT-DAO/youbei-api/data/entities"
	"github.com/ENFT-DAO/youbei-api/services"
	"github.com/ENFT-DAO/youbei-api/storage"
	"github.com/btcsuite/btcutil/bech32"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type CollectionIndexer struct {
	DeployerAddr string `json:"deployerAddr"`
	ElrondAPI    string `json:"elrondApi"`
	ElrondAPISec string `json:"elrondApiSec"`
	Logger       *log.Logger
	Delay        time.Duration // delay per request in second
}

func NewCollectionIndexer(deployerAddr string, elrondAPI string, elrondAPISec string, delay uint64) (*CollectionIndexer, error) {
	l := log.New(os.Stderr, "", log.LUTC|log.LstdFlags|log.Lshortfile)
	return &CollectionIndexer{
		DeployerAddr: deployerAddr,
		ElrondAPI:    elrondAPI,
		ElrondAPISec: elrondAPISec,
		Delay:        time.Duration(delay),
		Logger:       l}, nil
}

func (ci *CollectionIndexer) StartWorker() {
	logErr := ci.Logger
	var colsToCheck []dtos.CollectionToCheck
	api := ci.ElrondAPI
	for {
	deployLoop:
		var foundDeployedContracts uint64 = 0
		time.Sleep(time.Second * ci.Delay)
		deployerStat, err := storage.GetDeployerStat(ci.DeployerAddr)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				deployerStat, err = storage.CreateDeployerStat(ci.DeployerAddr)
				if err != nil {
					logErr.Println(err.Error())
					logErr.Println("something went wrong creating marketstat")
				}
			}
		}
		url := fmt.Sprintf("%s/accounts/%s/transactions?from=%d&withScResults=true&withLogs=false&order=asc",
			api,
			ci.DeployerAddr,
			deployerStat.LastIndex)
		res, err := services.GetResponse(url)

		if err != nil {
			logErr.Println(err.Error())
			logErr.Println(url)
			continue
		}

		var deployerTxs []entities.TransactionBC
		err = json.Unmarshal(res, &deployerTxs)
		if err != nil {
			logErr.Println(err.Error())
			logErr.Println("error unmarshal nfts deployer")
			continue
		}

		foundDeployedContracts += uint64(len(deployerTxs))
		for _, colR := range deployerTxs {
			if colR.Action.Name == "" {
				continue
			}
			name := colR.Action.Name
			if name == "deployNFTTemplateContract" && colR.Status != "fail" {
				if len(colR.Results) == 0 {
					goto deployLoop
				}
				mainDataStr := colR.Data
				mainData64Str, _ := base64.StdEncoding.DecodeString(mainDataStr)
				mainDatas := strings.Split(string(mainData64Str), "@")
				tokenIdHex := mainDatas[1]
				tokenIdStr, _ := hex.DecodeString(mainDatas[1])
				imageLink, _ := hex.DecodeString(mainDatas[4])
				metaLink, _ := hex.DecodeString(mainDatas[9])
				results := colR.Results
				result := results[0]
				data := result.Data
				decodedData64, _ := base64.StdEncoding.DecodeString(data)
				decodedData := strings.Split(string(decodedData64), "@")
				hexByte, err := hex.DecodeString(decodedData[2])
				if err != nil {
					logErr.Println(err.Error())
					continue
				}
				byte32, err := bech32.ConvertBits(hexByte, 8, 5, true)
				if err != nil {
					logErr.Println(err.Error())
					continue
				}
				bech32Addr, err := bech32.Encode("erd", byte32)
				if err != nil {
					logErr.Println(err.Error())
					continue
				}
				colsToCheck = append(colsToCheck, dtos.CollectionToCheck{CollectionAddr: bech32Addr, TokenID: string(tokenIdStr)})
				tokenId, err := hex.DecodeString(tokenIdHex)
				if err != nil {
					logErr.Println(err.Error())
					continue
				}

				dbCol, err := storage.GetCollectionByTokenId(string(tokenId))
				if err != nil {
					logErr.Println(err.Error())
					continue
				}

				dbCol.MetaDataBaseURI = string(metaLink)
				dbCol.TokenBaseURI = string(imageLink)
				metaInfoByte, err := services.GetResponse(dbCol.MetaDataBaseURI + "/1.json")

				if err != nil {
					logErr.Println(err.Error())
					continue
				}

				metaInfo := map[string]interface{}{}
				err = json.Unmarshal(metaInfoByte, &metaInfo)
				if err != nil {
					logErr.Println(err.Error())
					continue
				}

				dbCol.Description = metaInfo["description"].(string)
				err = storage.UpdateCollection(dbCol)
				if err != nil {
					logErr.Println(err.Error())
					continue
				}
				// get collection tx and check mint transactions
				_, err = storage.GetCollectionIndexer(bech32Addr)
				if err != nil {
					if err == gorm.ErrRecordNotFound {
						_, err = storage.CreateCollectionStat(entities.CollectionIndexer{CollectionName: string(tokenIdStr), CollectionAddr: bech32Addr})
						if err != nil {
							logErr.Println(err.Error())
							continue
						} else {
							continue
						}
					}
				}

			}

		}
		newStat, err := storage.UpdateDeployerIndexer(deployerStat.LastIndex+foundDeployedContracts, ci.DeployerAddr)
		if err != nil {
			logErr.Println(err.Error())
			logErr.Println("error update deployer index nfts ")
			continue
		}
		if newStat.LastIndex < deployerStat.LastIndex {
			logErr.Println("error something went wrong updating last index of deployer  ")
			continue
		}
		cols, err := storage.GetVerifiedCollections()
		if err != nil {
			logErr.Println(err.Error())
			continue
		}
		for _, colObj := range cols {
			col, err := storage.GetCollectionByTokenId(colObj.TokenID)
			if err != nil {
				logErr.Println("GetCollectionByTokenId", err.Error(), colObj.TokenID)
				continue
			}
			if colObj.ContractAddress == "" {
				colDetail, err := services.GetCollectionDetailBC(col.TokenID, api)
				if err != nil {
					continue
				}
				var address string
				for _, role := range colDetail.Roles {
					rolesStr, ok := role["roles"].([]interface{})
					if ok {
						for _, roleStr := range rolesStr {
							if strings.EqualFold(roleStr.(string), "ESDTRoleNFTCreate") {
								address = role["address"].(string)
							}
						}
					}
				}
				colObj.ContractAddress = address
				colObj.Name = colDetail.Name
				err = services.UpdateCollectionWithAddress(&colObj, map[string]interface{}{
					"Name":            colObj.Name,
					"ContractAddress": colObj.ContractAddress,
				})
				if err != nil {
					continue
				}
			}
			collectionIndexer, err := storage.GetCollectionIndexer(colObj.ContractAddress)
			if err != nil {
				if err == gorm.ErrRecordNotFound {
					collectionIndexer, err = storage.CreateCollectionStat(entities.CollectionIndexer{
						CollectionAddr: colObj.ContractAddress,
						CollectionName: colObj.TokenID,
					})
					if err != nil {
						logErr.Println(err.Error())
						logErr.Println("error create colleciton indexer")
						continue
					}
				} else {
					logErr.Println(err.Error())
					logErr.Println("error getting collection indexer")
					continue
				}
			}
			lastIndex := 0
			var lastNonce uint64 = 0
			for {
				url := fmt.Sprintf("%s/nftsFromCollection?collection=%s&from=%d",
					api,
					collectionIndexer.CollectionName,
					lastIndex)
				res, err := services.GetResponse(url)
				if err != nil {
					logErr.Println(err.Error())
					logErr.Println(err.Error())
					logErr.Println("error creating request for get nfts deployer")
					if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "deadline") {
						time.Sleep(time.Second * 10)
						continue
					}
				}

				var tokens []entities.TokenBC
				err = json.Unmarshal(res, &tokens)
				if err != nil {
					logErr.Println(err.Error(), "collection name ", collectionIndexer.CollectionName, "lastIndex", lastIndex)
					logErr.Println("error unmarshal nfts deployer")
					continue
				}
				if len(tokens) == 0 {
					continue
				}
				for _, token := range tokens {
					if collectionIndexer.LastNonce == token.Nonce {
						goto endLoop
					}
					imageURI, attributeURI := services.GetTokenBaseURIs(token)

					nonce10Str := strconv.FormatUint(token.Nonce, 10)

					nonceStr := strconv.FormatUint(token.Nonce, 16)
					if len(nonceStr)%2 != 0 {
						nonceStr = "0" + nonceStr
					}

					if strings.Contains(api, "devnet") {
						imageURI = strings.Replace(imageURI, "https://gateway.pinata.cloud/ipfs/", "https://devnet-media.elrond.com/nfts/asset/", 1)
					} else {
						imageURI = strings.Replace(imageURI, "https://gateway.pinata.cloud/ipfs/", "https://media.elrond.com/nfts/asset/", 1)
						imageURI = strings.Replace(imageURI, "https://ipfs.io/ipfs/", "https://media.elrond.com/nfts/asset/", 1)
					}
					if imageURI != "" {
						if string(imageURI[len(imageURI)-1]) == "/" {
							imageURI = imageURI[:len(imageURI)-1]
						}
						if strings.Contains(imageURI, "ipfs://") {
							imageURI = strings.Replace(imageURI, "ipfs://", "", 1)
							imageURI = ""
						}
					}

					if strings.Contains(strings.ToLower(imageURI), ".PNG") || strings.Contains(strings.ToLower(imageURI), ".JPG") || strings.Contains(strings.ToLower(imageURI), ".JPEG") {

					} else {
						imageURI = imageURI + "/" + nonce10Str + ".png"
					}

					youbeiMeta := strings.Replace(attributeURI, "https://gateway.pinata.cloud/ipfs/", "https://media.elrond.com/nfts/asset/", 1)
					youbeiMeta = strings.Replace(youbeiMeta, "https://ipfs.io/ipfs/", "https://media.elrond.com/nfts/asset/", 1)
					youbeiMeta = strings.Replace(youbeiMeta, "https://ipfs.io/ipfs/", "https://media.elrond.com/nfts/asset/", 1)
					youbeiMeta = strings.Replace(youbeiMeta, "ipfs://", "https://media.elrond.com/nfts/asset/", 1)
					if youbeiMeta != "" {
						if string(youbeiMeta[len(youbeiMeta)-1]) == "/" {
							youbeiMeta = youbeiMeta[:len(youbeiMeta)-1]
						}
						if strings.Contains(attributeURI, "ipfs://") {
							youbeiMeta = strings.Replace(youbeiMeta, "ipfs://", "", 1)
							youbeiMeta = ""
						}
					}
					url := fmt.Sprintf("%s/%s.json", youbeiMeta, nonce10Str)
					attrbs, err := services.GetResponse(url)

					metadataJSON := make(map[string]interface{})
					err = json.Unmarshal(attrbs, &metadataJSON)
					if err != nil {
						logErr.Println(err.Error(), string(url), token.Collection, token.Attributes, token.Identifier, token.Media, token.Metadata)
					}
					var attributes datatypes.JSON
					attributesBytes, err := json.Marshal(metadataJSON["attributes"])
					if err != nil {
						logErr.Println(err.Error())
						attributesBytes = []byte{}
					}
					err = json.Unmarshal(attributesBytes, &attributes)
					if err != nil {
						logErr.Println(err.Error())
					}

					//get owner of token from database TODO
					acc, err := storage.GetAccountByAddress(token.Owner)
					if err != nil {
						if err != gorm.ErrRecordNotFound {
							continue
						} else {
							name := services.RandomName()
							acc = &entities.Account{
								Address: token.Owner,
								Name:    name,
							}
							err := storage.AddAccount(acc)
							if err != nil {
								if !strings.Contains(err.Error(), "duplicate") {
									logErr.Println("CRITICAL can't create user")
									continue
								} else {
									acc, err = storage.GetAccountByAddress(token.Owner)
									if err != nil {
										logErr.Println("CRITICAL can't get user")
										continue
									}
								}
							}
						}
					}
					//try get token from database TODO
					dbToken, err := storage.GetTokenByTokenIdAndNonce(token.Collection, token.Nonce)
					if err != nil {
						if err != gorm.ErrRecordNotFound {
							continue
						} else {

						}
					}
					if dbToken == nil {
						dbToken = &entities.Token{}
					}
					err = storage.AddToken(&entities.Token{
						TokenID:      token.Collection,
						MintTxHash:   dbToken.MintTxHash,
						CollectionID: col.ID,
						Nonce:        token.Nonce,
						NonceStr:     nonceStr,
						MetadataLink: string(youbeiMeta) + "/" + nonce10Str + ".json",
						ImageLink:    string(imageURI),
						TokenName:    token.Name,
						Attributes:   attributes,
						OwnerId:      acc.ID,
						OnSale:       false,
						PriceString:  dbToken.PriceString,
						PriceNominal: dbToken.PriceNominal,
					})
					if err != nil {
						logErr.Println(err.Error())
					}
				}
				lastNonce = tokens[0].Nonce

			endLoop:
				lastIndex += len(tokens)
				if collectionIndexer.LastNonce < lastNonce {
					err = storage.UpdateCollectionndexerWhere(&collectionIndexer,
						map[string]interface{}{
							"LastNonce": lastNonce,
						},
						"id=?",
						collectionIndexer.ID)
					if err != nil {
						logErr.Println("CRITICAL", err.Error())
					}
				}
			}
		}

	}

}
