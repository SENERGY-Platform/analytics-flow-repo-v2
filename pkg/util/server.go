/*
 * Copyright 2025 InfAI (CC SES)
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package util

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"
)

func NewServer(handler http.Handler, port int) *http.Server {
	return &http.Server{
		Addr:    ":" + strconv.FormatInt(int64(port), 10),
		Handler: handler,
	}
}

func StartServer(server *http.Server) error {
	Logger.Info("starting http server")
	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func StopServer(ctx context.Context, server *http.Server) error {
	defer Logger.Info("http server halted")
	<-ctx.Done()
	Logger.Info("stopping http server")
	ctxWt, cf := context.WithTimeout(context.Background(), time.Second*5)
	defer cf()
	if err := server.Shutdown(ctxWt); err != nil {
		return err
	}
	return nil
}
