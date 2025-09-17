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

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"syscall"

	"github.com/SENERGY-Platform/analytics-flow-repo-v2/pkg/api"
	"github.com/SENERGY-Platform/analytics-flow-repo-v2/pkg/config"
	operator_api "github.com/SENERGY-Platform/analytics-flow-repo-v2/pkg/operator-api"
	"github.com/SENERGY-Platform/analytics-flow-repo-v2/pkg/repo"
	"github.com/SENERGY-Platform/analytics-flow-repo-v2/pkg/util"
	sb_logger "github.com/SENERGY-Platform/go-service-base/logger"
	srv_info_hdl "github.com/SENERGY-Platform/mgw-go-service-base/srv-info-hdl"
	sb_util "github.com/SENERGY-Platform/mgw-go-service-base/util"
	permV2Client "github.com/SENERGY-Platform/permissions-v2/pkg/client"
)

var version string = "dev"

func main() {
	srvInfoHdl := srv_info_hdl.New("analytics-flow-repo-v2", version)

	ec := 0
	defer func() {
		os.Exit(ec)
	}()

	util.ParseFlags()

	cfg, err := config.New(util.Flags.ConfPath)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		ec = 1
		return
	}

	logFile, err := util.InitLogger(cfg.Logger)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		var logFileError *sb_logger.LogFileError
		if errors.As(err, &logFileError) {
			ec = 1
			return
		}
	}
	if logFile != nil {
		defer logFile.Close()
	}

	util.StructLogger = util.InitStructLogger(cfg.Logger)

	util.StructLogger.Info(srvInfoHdl.GetName(), "version", srvInfoHdl.GetVersion())
	util.StructLogger.Info("config: " + sb_util.ToJsonStr(cfg))

	err = repo.InitDB(cfg.MongoUrl)
	if err != nil {
		util.StructLogger.Error("error on db init", "error", err)
		ec = 1
		return
	}
	util.StructLogger.Debug("connected to database")
	defer repo.CloseDB()

	ctx, cf := context.WithCancel(context.Background())
	var perm permV2Client.Client

	if cfg.PermissionsV2Url == "mock" {
		util.StructLogger.Debug("using mock permissions")
		perm, err = permV2Client.NewTestClient(ctx)
	} else {
		perm = permV2Client.New(cfg.PermissionsV2Url)
	}
	operatorRepo := operator_api.New(cfg.OperatorRepoUrl)
	srv, err := repo.New(srvInfoHdl, perm, operatorRepo)
	if err != nil {
		util.StructLogger.Error("error on new repo", "error", err)
		ec = 1
		return
	}

	httpHandler, err := api.New(srv, map[string]string{
		api.HeaderApiVer:  srvInfoHdl.GetVersion(),
		api.HeaderSrvName: srvInfoHdl.GetName(),
	}, cfg.URLPrefix)
	if err != nil {
		util.StructLogger.Error("error on new httpHandler", "error", err)
		ec = 1
		return
	}

	httpServer := util.NewServer(httpHandler, cfg.ServerPort)

	go func() {
		util.WaitForSignal(ctx, syscall.SIGINT, syscall.SIGTERM)
		cf()
	}()

	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err = util.StartServer(httpServer); err != nil {
			util.StructLogger.Error("error on server start", "error", err)
			ec = 1
		}
		cf()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err = util.StopServer(ctx, httpServer); err != nil {
			util.StructLogger.Error("error on server stop", "error", err)
			ec = 1
		}
	}()

	wg.Wait()
}
