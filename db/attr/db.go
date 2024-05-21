package attr

import (
	"attribute-db/dataQ"
	"attribute-db/db/attr/data"
	"attribute-db/db/levelDB"
	"attribute-db/logging"
	"attribute-db/rest/router"
	"attribute-db/s3"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type DB struct {
	Data data.DATA
}

type Result struct {
	Status  string      `json:"status"`
	Data    interface{} `json:"data"`
	Meesage string      `json:"message"`
}

func (r *Result) GetByteData() []byte {
	data, _ := json.Marshal(r)
	return data
}

var gAttributeDBQueue *dataQ.Queue

func (db *DB) GetKey() string {
	if db.Data.Key == "" {
		jsonData, _ := json.Marshal(db.Data.Data)
		hash := sha256.Sum256(jsonData)
		db.Data.Key = hex.EncodeToString(hash[:])
	}
	return db.Data.Key
}

func extractNestedAttributes(value interface{}, prefix string) []string {
	var attributes []string
	// 맵 타입인 경우, 모든 키를 순회합니다.
	if subMap, ok := value.(map[string]interface{}); ok {
		for subKey, subValue := range subMap {
			// 재귀적으로 중첩된 속성을 탐색합니다.
			nestedAttributes := extractNestedAttributes(subValue, prefix+subKey+"/")
			attributes = append(attributes, nestedAttributes...)
		}
	} else if subArray, ok := value.([]string); ok {
		for _, subValue := range subArray {
			attributes = append(attributes, prefix+subValue)
		}
	} else if subString, ok := value.(string); ok {
		attributes = append(attributes, prefix+subString)
	} else {
		attributes = append(attributes, prefix[:len(prefix)-1]) // 마지막 '/' 제거
	}

	return attributes
}

func (db *DB) ExtractAttributePath() []string {
	attributes := make([]string, 0)
	for _, attr := range db.Data.Attribute {
		if attr != "" {
			if value, exists := db.Data.Data[attr]; exists {
				attr = db.Data.Root + "/" + attr
				switch value.(type) {
				case []interface{}:
					for _, subValue := range value.([]interface{}) {
						attributes = append(attributes, extractNestedAttributes(subValue.(string), attr+"/")...)
					}
				case interface{}:
					attributes = append(attributes, extractNestedAttributes(value.(string), attr+"/")...)
				default:
					logging.PrintERROR(fmt.Sprintf("타입이 맞지 않습니다: %+v\n", value))
				}
			} else {
				logging.PrintERROR(fmt.Sprintf("특정 속성이 존재하지 않습니다: %+v\n", db.Data))
			}
		}
	}
	return attributes
}

func Init(scheme string, port int, s3conn s3.S3) *router.Router {
	if gAttributeDBQueue == nil {
		gAttributeDBQueue = dataQ.NewQueue()
	}
	r := router.NewRouter("/attrdb")
	r.SetScheme(router.Scheme(scheme)).SetPort(fmt.Sprintf(":%d", port))
	r.NewSubRouter(`/search`).SetHandler(router.POST, func(w http.ResponseWriter, r *http.Request) {
		var err error
		response := Result{}
		defer func() {
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				response.Status = "ERROR"
				response.Meesage = err.Error()
				w.Write(response.GetByteData())
			}
		}()

		if body, err := io.ReadAll(r.Body); err == nil {
			logging.PrintINFO(fmt.Sprintf("Search request : %s", string(body)))
			request := DB{}
			defer r.Body.Close()
			if err := json.Unmarshal(body, &request.Data); err == nil {
				if request.Data.CallBack == "" {
					attrList := request.ExtractAttributePath()

					ldb := levelDB.GetDB(fmt.Sprintf("%s.db", request.Data.Root))
					if attrList != nil {
						strList := make([]string, 0)
						result := make(map[string]interface{})

						if dbg, err := ldb.GetAll(); err != nil {
							fmt.Println(err)
						} else {
							fmt.Println(len(dbg))
							for _, value := range dbg {
								fmt.Println(value)
							}
						}

						for _, attr := range attrList {

							if attrInterface, err := ldb.Get(attr); err != nil {
								if objList, err := s3conn.GetObjectList(attr); err == nil {
									for _, obj := range objList {
										if _, ok := result[obj]; !ok {
											result[obj] = ""
											strList = append(strList, obj)
										}
									}
								}
							} else {
								fmt.Println(attrInterface)
								var attrMap map[string]interface{}
								json.Unmarshal(attrInterface, &attrMap)
								for key, _ := range attrMap {
									if _, ok := result[key]; !ok {
										strList = append(strList, key)
									}
								}
							}
						}

						w.WriteHeader(http.StatusOK)
						response.Status = "OK"
						response.Data = strList
						w.Write(response.GetByteData())
					}
				} else {
					gAttributeDBQueue.Push(request)
					w.WriteHeader(http.StatusOK)
					response.Status = "OK"
					response.Meesage = "콜백 처리 예정"
					w.Write(response.GetByteData())
				}
			}
		}
	})
	r.NewSubRouter(`/upload`).SetHandler(router.POST, func(w http.ResponseWriter, r *http.Request) {
		var err error
		response := Result{}
		defer func() {
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				response.Status = "ERROR"
				response.Meesage = err.Error()
				w.Write(response.GetByteData())
			}
		}()

		contentType := r.Header.Get("Content-Type")
		if strings.Contains(contentType, "application/json") {
			if body, err := io.ReadAll(r.Body); err == nil {
				var request DB
				defer r.Body.Close()
				if err := json.Unmarshal(body, &request.Data); err == nil {
					if request.Data.CallBack == "" {
						request.GetKey()
						logging.PrintINFO(fmt.Sprintf("키값 : %s", request.Data.Key))
						attrList := request.ExtractAttributePath()
						if attrList != nil {
							dataRooWithKey := fmt.Sprintf("%s/room/%s", request.Data.Root, request.Data.Key)
							dataRoom := fmt.Sprintf("%s/room", request.Data.Root)
							byteData, _ := json.Marshal(request.Data.Data)

							ldb := levelDB.GetDB(fmt.Sprintf("%s.db", request.Data.Root))
							deleteList := make([]string, 0)
							createList := make([]string, 0)
							if rawData, err := ldb.Get(dataRoom); err == nil {
								attrMap := map[string]DB{}
								json.Unmarshal(rawData, &attrMap)

								if attrData, ok := attrMap[request.Data.Key]; ok {
									orgAttrList := attrData.ExtractAttributePath()

									orgAttrMap := make(map[string]bool)
									newAttrMap := make(map[string]bool)
									for _, attr := range orgAttrList {
										orgAttrMap[attr] = true
									}

									for _, attr := range attrList {
										newAttrMap[attr] = true
									}

									for _, attr := range orgAttrList {
										if !newAttrMap[attr] {
											deleteList = append(deleteList, attr)
										}
									}

									for _, attr := range attrList {
										if !orgAttrMap[attr] {
											createList = append(createList, attr)
										}
									}
								} else {
									createList = attrList
								}
							} else {
								createList = attrList
							}

							if err := s3conn.Write(byteData, dataRooWithKey+"/data"); err == nil {
								if roomBytes, err := ldb.Get(dataRoom); err == nil {
									roomMap := map[string]DB{}
									json.Unmarshal(roomBytes, &roomMap)

									roomMap[request.Data.Key] = request
									ldb.Put(dataRoom, roomMap)
								} else {
									roomMap := make(map[string]DB)
									roomMap[request.Data.Key] = request
									ldb.Put(dataRoom, roomMap)
								}

								for _, attr := range createList {
									if err := s3conn.Write([]byte(request.Data.Key), fmt.Sprintf("%s/%s", attr, request.Data.Key)); err != nil {
										if err := s3conn.Write([]byte(request.Data.Key), fmt.Sprintf("%s/%s", attr, request.Data.Key)); err != nil {
											logging.PrintERROR(err.Error())
										} else {
											if attrInterface, err := ldb.Get(attr); err == nil {
												var attrMap map[string]bool
												json.Unmarshal(attrInterface, &attrMap)
												attrMap[request.Data.Key] = true
												ldb.Put(attr, attrMap)
											} else {
												attrMap := make(map[string]bool)
												attrMap[request.Data.Key] = true
												ldb.Put(attr, attrMap)
											}
										}
									} else {
										if attrInterface, err := ldb.Get(attr); err == nil {
											var attrMap map[string]bool
											json.Unmarshal(attrInterface, &attrMap)
											attrMap[request.Data.Key] = true
											ldb.Put(attr, attrMap)
										} else {
											attrMap := make(map[string]bool)
											attrMap[request.Data.Key] = true
											ldb.Put(attr, attrMap)
										}
									}
								}

								for _, attr := range deleteList {
									if err := s3conn.DeleteObject(fmt.Sprintf("%s/%s", attr, request.Data.Key)); err != nil {
										if err := s3conn.DeleteObject(fmt.Sprintf("%s/%s", attr, request.Data.Key)); err != nil {
											logging.PrintERROR(err.Error())
										} else {
											if attrInterface, err := ldb.Get(attr); err == nil {
												var attrMap map[string]bool
												json.Unmarshal(attrInterface, &attrMap)
												delete(attrMap, request.Data.Key)
												ldb.Delete(attr)
											}
										}
									} else {
										if attrInterface, err := ldb.Get(attr); err == nil {
											var attrMap map[string]bool
											json.Unmarshal(attrInterface, &attrMap)
											delete(attrMap, request.Data.Key)
											ldb.Delete(attr)
										}
									}
								}

								w.WriteHeader(http.StatusOK)
								response.Status = "OK"
								response.Data = request
								w.Write(response.GetByteData())
							}
						}
					} else {
						gAttributeDBQueue.Push(request)
						w.WriteHeader(http.StatusOK)
						response.Status = "OK"
						response.Meesage = "콜백 처리 예정"
						w.Write(response.GetByteData())
					}
				}
			}
		} else if strings.Contains(contentType, "multipart/form-data") {
			// 멀티파트 데이터 파싱
			if err := r.ParseMultipartForm(10 << 20); err == nil {
				// JSON 데이터가 담긴 'data' 필드 처리
				jsonData := r.FormValue("json")
				request := DB{}
				logging.PrintINFO(fmt.Sprintf("Multi-part JSON : %s", jsonData))
				if err := json.Unmarshal([]byte(jsonData), &request.Data); err == nil {
					if request.Data.CallBack == "" {
						request.GetKey()
						attrList := request.ExtractAttributePath()
						if attrList != nil {
							dataRooWithKey := fmt.Sprintf("%s/room/%s", request.Data.Root, request.Data.Key)
							dataRoom := fmt.Sprintf("%s/room", request.Data.Root)

							byteData, _ := json.Marshal(request.Data.Data)

							ldb := levelDB.GetDB(fmt.Sprintf("%s.db", request.Data.Root))
							deleteList := make([]string, 0)
							createList := make([]string, 0)
							if rawData, err := ldb.Get(dataRoom); err == nil {
								attrMap := map[string]DB{}
								json.Unmarshal(rawData, &attrMap)

								if attrData, ok := attrMap[request.Data.Key]; ok {
									orgAttrList := attrData.ExtractAttributePath()

									orgAttrMap := make(map[string]bool)
									newAttrMap := make(map[string]bool)
									for _, attr := range orgAttrList {
										orgAttrMap[attr] = true
									}

									for _, attr := range attrList {
										newAttrMap[attr] = true
									}

									for _, attr := range orgAttrList {
										if !newAttrMap[attr] {
											deleteList = append(deleteList, attr)
										}
									}

									for _, attr := range attrList {
										if !orgAttrMap[attr] {
											createList = append(createList, attr)
										}
									}
								} else {
									createList = attrList
								}
							} else {
								createList = attrList
							}

							if err := s3conn.Write(byteData, dataRooWithKey+"/data"); err == nil {
								if roomBytes, err := ldb.Get(dataRoom); err == nil {
									roomMap := map[string]DB{}
									json.Unmarshal(roomBytes, &roomMap)
									roomMap[request.Data.Key] = request
									ldb.Put(dataRoom, roomMap)
								} else {
									roomMap := make(map[string]DB)
									roomMap[request.Data.Key] = request
									ldb.Put(dataRoom, roomMap)
								}

								if file, _, _ := r.FormFile("image"); file != nil {
									defer file.Close()
									buffer := bytes.NewBuffer(nil)
									if _, err := buffer.ReadFrom(file); err == nil {
										s3conn.WriteImage(buffer.Bytes(), dataRooWithKey+"/img")
									}
								}

								for _, attr := range createList {
									if err := s3conn.Write([]byte(request.Data.Key), fmt.Sprintf("%s/%s", attr, request.Data.Key)); err != nil {
										if err := s3conn.Write([]byte(request.Data.Key), fmt.Sprintf("%s/%s", attr, request.Data.Key)); err != nil {
											logging.PrintERROR(err.Error())
										} else {
											if attrInterface, err := ldb.Get(attr); err == nil {
												var attrMap map[string]bool
												json.Unmarshal(attrInterface, &attrMap)
												attrMap[request.Data.Key] = true
												ldb.Put(attr, attrMap)
											} else {
												attrMap := make(map[string]bool)
												attrMap[request.Data.Key] = true
												ldb.Put(attr, attrMap)
											}
										}
									} else {
										if attrInterface, err := ldb.Get(attr); err == nil {
											var attrMap map[string]bool
											json.Unmarshal(attrInterface, &attrMap)
											attrMap[request.Data.Key] = true
											ldb.Put(attr, attrMap)
										} else {
											attrMap := make(map[string]bool)
											attrMap[request.Data.Key] = true
											ldb.Put(attr, attrMap)
										}
									}
								}

								for _, attr := range deleteList {
									if err := s3conn.DeleteObject(fmt.Sprintf("%s/%s", attr, request.Data.Key)); err != nil {
										if err := s3conn.DeleteObject(fmt.Sprintf("%s/%s", attr, request.Data.Key)); err != nil {
											logging.PrintERROR(err.Error())
										} else {
											if attrInterface, err := ldb.Get(attr); err == nil {
												var attrMap map[string]bool
												json.Unmarshal(attrInterface, &attrMap)
												delete(attrMap, request.Data.Key)
												ldb.Delete(attr)
											}
										}
									} else {
										if attrInterface, err := ldb.Get(attr); err == nil {
											var attrMap map[string]bool
											json.Unmarshal(attrInterface, &attrMap)
											delete(attrMap, request.Data.Key)
											ldb.Delete(attr)
										}
									}
								}

								w.WriteHeader(http.StatusOK)
								response.Status = "OK"
								response.Data = request
								w.Write(response.GetByteData())
							}
						}
					} else {
						gAttributeDBQueue.Push(request)
						w.WriteHeader(http.StatusOK)
						response.Status = "OK"
						response.Meesage = "콜백 처리 예정"
						w.Write(response.GetByteData())
					}
				} else {
					w.WriteHeader(http.StatusOK)
					response.Status = "ERROR"
					response.Meesage = err.Error()
					w.Write(response.GetByteData())
				}
			} else {
				w.WriteHeader(http.StatusOK)
				response.Status = "ERROR"
				response.Meesage = err.Error()
				w.Write(response.GetByteData())
			}
		}
	})
	r.NewSubRouter(`/get`).SetHandler(router.POST, func(w http.ResponseWriter, r *http.Request) {
		var err error
		response := Result{}
		defer func() {
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				response.Status = "ERROR"
				response.Meesage = err.Error()
				w.Write(response.GetByteData())
			}
		}()

		if body, err := io.ReadAll(r.Body); err == nil {
			var request DB
			defer r.Body.Close()
			if err := json.Unmarshal(body, &request.Data); err == nil {
				if request.Data.CallBack == "" {
					ldb := levelDB.GetDB(fmt.Sprintf("%s.db", request.Data.Root))
					dataRooWithKey := fmt.Sprintf("%s/room/%s", request.Data.Root, request.Data.Key)
					dataRoom := fmt.Sprintf("%s/room", request.Data.Root)

					if d, err := ldb.Get(dataRoom); err != nil {
						if d, err := s3conn.Download(dataRooWithKey + "data.json"); err == nil {
							dData := data.DATA{}
							json.Unmarshal(d, &dData)
							w.WriteHeader(http.StatusOK)
							response.Status = "OK1"
							response.Data = dData
							w.Write(response.GetByteData())
						}
					} else {
						roomMap := map[string]DB{}
						json.Unmarshal(d, &roomMap)
						logging.PrintINFO(fmt.Sprintf("%#v\n", request))
						if d, ok := roomMap[request.Data.Key]; ok {
							w.WriteHeader(http.StatusOK)
							response.Status = "OK2"
							response.Data = d
							w.Write(response.GetByteData())
						} else {
							if d, err := s3conn.Download(dataRooWithKey + "data.json"); err == nil {
								dData := data.DATA{}
								json.Unmarshal(d, &dData)
								w.WriteHeader(http.StatusOK)
								response.Status = "OK1"
								response.Data = dData
								w.Write(response.GetByteData())
							} else {
								w.WriteHeader(http.StatusOK)
								response.Status = "ERROR"
								response.Data = err.Error()
								w.Write(response.GetByteData())
							}
						}
					}
				} else {
					gAttributeDBQueue.Push(request)
					w.WriteHeader(http.StatusOK)
					response.Status = "OK"
					response.Meesage = "콜백 처리 예정"
					w.Write(response.GetByteData())
				}
			}
		}
	})
	r.NewSubRouter(`/delete`).SetHandler(router.POST, func(w http.ResponseWriter, r *http.Request) {
		var err error
		response := Result{}
		defer func() {
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				response.Status = "ERROR"
				response.Meesage = err.Error()
				w.Write(response.GetByteData())
			}
		}()

		if body, err := io.ReadAll(r.Body); err == nil {
			var request DB
			defer r.Body.Close()
			if err := json.Unmarshal(body, &request.Data); err == nil {
				if request.Data.Key != "" {
					if request.Data.CallBack == "" {
						delAttrList := make([]string, 0)
						dataRooWithKey := fmt.Sprintf("%s/room/%s", request.Data.Root, request.Data.Key)
						dataRoom := fmt.Sprintf("%s/room", request.Data.Root)

						ldb := levelDB.GetDB(fmt.Sprintf("%s.db", request.Data.Root))
						if rawData, err := ldb.Get(dataRoom); err == nil {
							attrMap := map[string]DB{}
							json.Unmarshal(rawData, &attrMap)
							if attrData, ok := attrMap[request.Data.Key]; ok {
								json.Unmarshal(rawData, &attrData)
								delAttrList = attrData.ExtractAttributePath()
							} else {
								delAttrList = request.ExtractAttributePath()
							}
						} else {
							delAttrList = request.ExtractAttributePath()
						}

						if err := s3conn.DeleteObject(dataRooWithKey + "/data.json"); err == nil {
							s3conn.DeleteObject(dataRooWithKey + "/img.jpg")
							if rawData, err := ldb.Get(dataRoom); err == nil {
								attrMap := map[string]DB{}
								json.Unmarshal(rawData, &attrMap)
								if _, ok := attrMap[request.Data.Key]; ok {
									delete(attrMap, request.Data.Key)
									ldb.Update(dataRoom, attrMap)
								}
							}

							for _, attr := range delAttrList {
								if err := s3conn.DeleteObject(fmt.Sprintf("%s/%s", attr, request.Data.Key)); err != nil {
									if err := s3conn.DeleteObject(fmt.Sprintf("%s/%s", attr, request.Data.Key)); err != nil {
										logging.PrintERROR(err.Error())
									} else {
										if attrInterface, err := ldb.Get(attr); err == nil {
											var attrMap map[string]bool
											json.Unmarshal(attrInterface, &attrMap)
											delete(attrMap, request.Data.Key)
											ldb.Delete(attr)
										}
									}
								} else {
									if attrInterface, err := ldb.Get(attr); err == nil {
										var attrMap map[string]bool
										json.Unmarshal(attrInterface, &attrMap)
										delete(attrMap, request.Data.Key)
										ldb.Delete(attr)
									}
								}
							}

							w.WriteHeader(http.StatusOK)
							response.Status = "OK"
							response.Data = request
							w.Write(response.GetByteData())
						}
					} else {
						gAttributeDBQueue.Push(request)
						w.WriteHeader(http.StatusOK)
						response.Status = "OK"
						response.Meesage = "콜백 처리 예정"
						w.Write(response.GetByteData())
					}
				}
			} else {
				w.WriteHeader(http.StatusBadRequest)
				response.Status = "ERROR"
				response.Meesage = "Unknowon Key"
				w.Write(response.GetByteData())
			}
		}
	})
	r.NewSubRouter(`/get_all`).SetHandler(router.POST, func(w http.ResponseWriter, r *http.Request) {
		var err error
		response := Result{}
		defer func() {
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				response.Status = "ERROR"
				response.Meesage = err.Error()
				w.Write(response.GetByteData())
			}
		}()

		if body, err := io.ReadAll(r.Body); err == nil {
			var request DB
			defer r.Body.Close()
			if err := json.Unmarshal(body, &request.Data); err == nil {
				if request.Data.CallBack == "" {
					ldb := levelDB.GetDB(fmt.Sprintf("%s.db", request.Data.Root))
					dataRoom := fmt.Sprintf("%s/room", request.Data.Root)
					if roomData, err := ldb.Get(dataRoom); err != nil {
						if roomData, err := s3conn.GetObjectList(dataRoom); err == nil {
							allList := make([]data.DATA, 0)
							for _, room := range roomData {
								if result, err := s3conn.Download(room + "/data.json"); err == nil {
									resultDB := DB{}
									json.Unmarshal(result, &resultDB)
									allList = append(allList, resultDB.Data)
								}
							}
							w.WriteHeader(http.StatusOK)
							response.Status = "OK1"
							response.Data = allList
							w.Write(response.GetByteData())
						}
					} else {
						attrMapList := make([]data.DATA, 0)
						roomDataMap := map[string]DB{}
						json.Unmarshal(roomData, &roomDataMap)
						for _, value := range roomDataMap {
							attrMapList = append(attrMapList, value.Data)
						}

						w.WriteHeader(http.StatusOK)
						response.Status = "OK2"
						response.Data = attrMapList
						w.Write(response.GetByteData())
					}
				} else {
					gAttributeDBQueue.Push(request)
					w.WriteHeader(http.StatusOK)
					response.Status = "OK"
					response.Meesage = "콜백 처리 예정"
					w.Write(response.GetByteData())
				}
			}
		}
	})
	r.Run()
	return r
}
