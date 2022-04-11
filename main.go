package main

import (
	"flag"
	"github.com/kubespace/pipeline-plugin/pkg/conf"
	"github.com/kubespace/pipeline-plugin/pkg/models"
	"github.com/kubespace/pipeline-plugin/pkg/models/mysql"
	"github.com/kubespace/pipeline-plugin/pkg/server"
	"github.com/kubespace/pipeline-plugin/pkg/utils"
	"k8s.io/klog"
)

var (
	port             = flag.Int("port", 8080, "Server port to listen.")
	dataDir          = flag.String("dataDir", "/tmp", "Data root dir to execute plugin")
	callbackEndpoint = flag.String("callbackEndpoint", "http://localhost:80", "Plugin callback to pipeline endpoint")
	callbackUrl      = flag.String("callbackUrl", "/api/v1/pipeline/callback", "Plugin callback to pipeline url")
	mysqlHost        = flag.String("mysql-host", "127.0.0.1:3306", "mysql address used.")
	mysqlUser        = flag.String("mysql-user", "root", "mysql db user.")
	mysqlPassword    = flag.String("mysql-password", "", "mysql password used.")
	mysqlDbName      = flag.String("mysql-dbname", "kubespace", "mysql db used.")
)

func main() {
	var err error
	klog.InitFlags(nil)
	flag.Parse()
	flag.VisitAll(func(flag *flag.Flag) {
		klog.Infof("FLAG: --%s=%q", flag.Name, flag.Value)
	})
	conf.AppConfig.DataDir = *dataDir
	conf.AppConfig.CallbackEndpoint = *callbackEndpoint
	conf.AppConfig.CallbackUrl = *callbackUrl
	conf.AppConfig.CallbackClient, err = utils.NewHttpClient(*callbackEndpoint)
	if err != nil {
		panic(err)
	}
	mysqlOptions := &mysql.Options{
		Username: *mysqlUser,
		Password: *mysqlPassword,
		Host:     *mysqlHost,
		DbName:   *mysqlDbName,
	}
	models.Models, err = models.NewModels(mysqlOptions)
	if err != nil {
		panic(err)
	}

	serverConfig := &server.Config{
		Port: *port,
	}
	pluginServer, err := server.NewServer(serverConfig)
	if err != nil {
		panic(err)
	}
	if err = pluginServer.Run(); err != nil {
		panic(err)
	}
}

//func main() {
//	// Create the remote with repository URL
//	//rem := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
//	//	Name: "origin",
//	//	URLs: []string{"https://github.com/Zenika/MARCEL"},
//	//})
//	//
//	//log.Print("Fetching tags...")
//	//
//	//// We can then use every Remote functions to retrieve wanted information
//	//refs, err := rem.List(&git.ListOptions{})
//	//if err != nil {
//	//	log.Fatal(err)
//	//}
//	//
//	//// Filters the references list and only keeps tags
//	//var tags []string
//	//for _, ref := range refs {
//	//	log.Println(ref)
//	//	if ref.Name().IsTag() {
//	//		tags = append(tags, ref.Name().Short())
//	//	}
//	//}
//	//
//	//if len(tags) == 0 {
//	//	log.Println("No tags!")
//	//	return
//	//}
//	//
//	//log.Printf("Tags found: %v", tags)
//	re, _ := regexp.Compile("git@([\\w\\.]+):[\\d]*/?([\\w/]+)[\\.git]*")
//	codeName := re.FindStringSubmatch("git@github.com:8990/kubespace/kubespace")
//	fmt.Println(codeName)
//}

//func main() {
//	// 建立SSH客户端连接
//	client, err := ssh.Dial("tcp", "148.153.72.88:22", &ssh.ClientConfig{
//		User:            "root",
//		Auth:            []ssh.AuthMethod{ssh.Password("DB-china123")},
//		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
//	})
//	if err != nil {
//		panic(err)
//	}
//
//	// 建立新会话
//	session, err := client.NewSession()
//	defer session.Close()
//	if err != nil {
//		panic(err)
//	}
//	session.Stdout = os.Stdout
//
//	err = session.Run("export HOME=/tmp; bash -cx 'pwd; ls ; ls /abc; sleep 5; echo $HOME' 2>&1")
//	if err != nil {
//		fmt.Printf("Failed to run command, Err:%s\n", err.Error())
//		os.Exit(0)
//	}
//	//fmt.Println(string(result))
//}
