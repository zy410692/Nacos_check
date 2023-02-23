package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/olekukonko/tablewriter"
	"github.com/tidwall/gjson"
	"gopkg.in/ini.v1"
	"io/ioutil"
	"nacos-check/pkg"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var mutex sync.Mutex

func (d *Nacos) GetJson(result_type string) (result interface{}, err error) {
	mutex.Lock()
	defer mutex.Unlock()
	defer func() {
		if func_err := recover(); func_err != nil {
			result = []byte("[]")
			err = errors.New("error")
		}
	}()
	d.GetNacosInstance()
	nacos_server := d.Clusterdata[d.Host]
	if len(nacos_server.HealthInstance) != 0 {
		var nacos NacosFile
		for _, na := range nacos_server.HealthInstance {
			var ta NacosTarget
			ta.Labels = make(map[string]string)
			for key, value := range ADDLABEL {
				ta.Labels[key] = value
			}
			ta.Targets = append(ta.Targets, na[2])
			ta.Labels["namespace"] = na[0]
			ta.Labels["service"] = na[1]
			ta.Labels["hostname"] = na[4]
			ta.Labels["weight"] = na[5]
			ta.Labels["pid"] = na[6]
			ta.Labels["ip"] = na[8]
			ta.Labels["port"] = na[9]
			ta.Labels["containerd"] = na[7]
			nacos.Data = append(nacos.Data, ta)
		}
		if result_type == "json" {
			result = nacos.Data
			return result, err
		}
		data, err := json.MarshalIndent(&nacos.Data, "", "  ")
		if err != nil {
			fmt.Println("json序列化失败!")
			result = []byte("[]")
			return result, err
		}
		result = data
		return result, err
	}
	result = []byte("[]")
	return result, err
}
func (d *Nacos) WriteFile() {
	var basedir string
	var filename string
	basedir = path.Dir(WRITEFILE)
	filename = path.Base(WRITEFILE)
	if err := os.MkdirAll(basedir, os.ModePerm); err != nil {
		os.Exit(EXITCODE)
	}
	file, err := os.OpenFile(basedir+"/.nacos_tmp.json", os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("创建文件失败 %s", err)
		os.Exit(2)
	}
	defer file.Close()
	jsondata, err := d.GetJson("byte")
	data := make([]byte, 0)
	var check bool
	if data, check = jsondata.([]byte); !check {
		fmt.Println("转换失败")
	}
	if _, err := file.Write(data); err != nil {
		fmt.Println("写入失败", err)
		os.Exit(EXITCODE)
	}
	file.Close()
	os.Rename(basedir+"/.nacos_tmp.json", basedir+"/"+filename)
	fmt.Println("写入成功:", basedir+"/"+filename)
}

func (d *Nacos) HttpReq(apiurl string) []byte {
	req, _ := http.NewRequest("GET", apiurl, nil)
	u, err := url.Parse(apiurl)
	res, err := d.Client.Do(req)
	if err != nil {
		panic(err)
	}
	if res.StatusCode != 200 {
		if res.StatusCode == 501 && u.Path == "/nacos/v1/ns/operator/servers" {
			//panic(fmt.Sprintf("单机不支持集群,请求状态码异常:%d", res.StatusCode))
			_url := fmt.Sprintf("%s/nacos/v2/core/cluster/node/list", d.DefaultUlr)
			return d.HttpReq(_url)
		}
		if res.StatusCode == 501 && u.Path == "/nacos/v1/ns/upgrade/ops/metrics" {
			panic(fmt.Sprintf("此版本不支持查看升级状态:%d", res.StatusCode))
		}
		panic(fmt.Sprintf("%s请求状态码异常:%d", apiurl, res.StatusCode))
	}
	defer res.Body.Close()
	resp, _ := ioutil.ReadAll(res.Body)
	return resp

}

func (d *Nacos) GetCluster() {
	_url := fmt.Sprintf("%s/nacos/v1/ns/operator/servers", d.DefaultUlr)
	res := d.HttpReq(_url)
	d.Cluster = string(res)
}

func (d *Nacos) GetNameSpace() {
	_url := fmt.Sprintf("%s/nacos/v1/console/namespaces", d.DefaultUlr)
	res := d.HttpReq(_url)
	err := json.Unmarshal(res, &d.Namespaces)
	if err != nil {
		fmt.Println("获取命名空间json异常")
	}
}
func (d *Nacos) GetService(namespaceId string) []byte {
	_url := fmt.Sprintf("%s/nacos/v1/ns/service/list?pageNo=1&pageSize=500&namespaceId=%s", d.DefaultUlr, namespaceId)
	res := d.HttpReq(_url)
	return res
}

func (d *Nacos) GetInstance(servicename string, namespaceId string) []byte {
	_url := fmt.Sprintf("%s/nacos/v1/ns/instance/list?serviceName=%s&namespaceId=%s", d.DefaultUlr, servicename, namespaceId)
	res := d.HttpReq(_url)
	return res
}

func (d *Nacos) GetV2Upgrade() []byte {
	_url := fmt.Sprintf("%s/nacos/v1/ns/upgrade/ops/metrics", d.DefaultUlr)
	res := d.HttpReq(_url)
	return res
}

func (d *Nacos) TableRender() {
	nacos_server := d.Clusterdata[d.Host]
	tabletitle := []string{"命名空间", "服务名称", "实例", "健康状态", "主机名", "权重", "PID", "容器"}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(tabletitle)
	for _, v := range nacos_server.HealthInstance {
		tabledata := v[0:8]
		if FIND == "" {
			table.Append(tabledata)
		} else {
			if strings.Contains(v[0], FIND) {
				table.Append(tabledata)
			}
			if strings.Contains(v[1], FIND) {
				table.Append(tabledata)
			}
			if strings.Contains(v[2], FIND) {
				table.Append(tabledata)
			}
		}
	}
	fmt.Printf("健康实例:(%d 个)\n", table.NumLines())
	table.Render()
	if len(nacos_server.UnHealthInstance) != 0 {
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader(tabletitle)
		for _, v := range d.Healthydataerr {
			if strings.Contains(v[0], FIND) {
				table.Append(v)
			}
			if strings.Contains(v[1], FIND) {
				table.Append(v)
			}
			if strings.Contains(v[2], FIND) {
				table.Append(v)
			}
		}
		fmt.Printf("异常实例:(%d 个)\n", table.NumLines())
		table.Render()
	}
}
func (d *Nacos) GetNacosInstance() {
	cluster_list := []string{d.Host}
	d.Clusterdata = make(map[string]ClusterStatus)
	if CLUSTER {
		d.GetCluster()
		var results []gjson.Result
		var leader gjson.Result
		if len(gjson.Get(string(d.Cluster), "servers").String()) != 0 {
			leader = gjson.Get(d.Cluster, "servers.0.extendInfo.raftMetaData.metaDataMap.naming_instance_metadata.leader")
			results = gjson.GetMany(d.Cluster, "servers.#.ip", "servers.#.port", "servers.#.state", "servers.#.extendInfo.version", "servers.#.extendInfo.lastRefreshTime")
		} else {
			leader = gjson.Get(d.Cluster, "data.0.extendInfo.raftMetaData.metaDataMap.naming_instance_metadata.leader")
			results = gjson.GetMany(d.Cluster, "data.#.ip", "data.#.port", "data.#.state", "data.#.extendInfo.version", "data.#.extendInfo.lastRefreshTime")
		}
		d.Leader = leader.String()
		for key := range results[0].Array() {
			timeStampStr := results[4].Array()[key].String()
			timeStamp, _ := strconv.Atoi(timeStampStr)
			formatTimeStr := time.Unix(int64(timeStamp/1000), 0).Format("2006-01-02 15:04:05")
			var cluster ClusterStatus
			cluster.Ip = results[0].Array()[key].String()
			cluster.Port = results[1].Array()[key].String()
			cluster.State = results[2].Array()[key].String()
			cluster.Version = results[3].Array()[key].String()
			cluster.LastRefreshTime = formatTimeStr
			key := fmt.Sprintf("%s:%s", results[0].Array()[key].String(), results[1].Array()[key].String())
			d.Clusterdata[key] = cluster
			cluster_list = append(cluster_list, key)
		}
	} else {
		var cluster ClusterStatus
		cluster.Ip = d.Host
		cluster.Port = d.Port
		cluster.State = "UP"
		cluster.Version = ""
		cluster.LastRefreshTime = ""
		key := fmt.Sprintf("%s:%s", d.Host, d.Port)
		d.Clusterdata[key] = cluster
		cluster_list = append(cluster_list, key)

	}
	if !CLUSTER {
		for _, server := range cluster_list {
			_url := fmt.Sprintf("%s://%s", d.Scheme, server)
			if _url == d.DefaultUlr {
				cluster_list = []string{server}
			}
		}
	}
	if !CLUSTER && len(cluster_list) != 1 {
		_url := fmt.Sprintf("%s", d.Host)
		cluster_list = []string{_url}
	}
	for _, server := range cluster_list {
		//NACOSURL = fmt.Sprintf("%s://%s", d.Scheme, server)
		d.GetNameSpace()
		for _, namespace := range d.Namespaces.Data {
			res := d.GetService(namespace.Namespace)
			var ser Service
			var cluster ClusterStatus
			cluster = d.Clusterdata[server]
			if V2UPGRADE {
				v2 := d.GetV2Upgrade()
				rep, _ := regexp.Compile(".*##.*")
				v2 = rep.ReplaceAll(v2, []byte(""))
				cfg, err := ini.Load(v2)
				if err != nil {
					fmt.Println("v2解析错误")
				}
				IsDoubleWriteEnabled, _ := cfg.Section("").Key("isDoubleWriteEnabled").Bool()
				Upgraded, _ := cfg.Section("").Key("upgraded").Bool()
				IsAll20XVersion, _ := cfg.Section("").Key("isAll20XVersion").Bool()
				ServiceCountV1, _ := cfg.Section("").Key("serviceCountV1").Int64()
				InstanceCountV1, _ := cfg.Section("").Key("instanceCountV1").Int64()
				ServiceCountV2, _ := cfg.Section("").Key("serviceCountV2").Int64()
				InstanceCountV2, _ := cfg.Section("").Key("instanceCountV2").Int64()
				SubscribeCountV2, _ := cfg.Section("").Key("subscribeCountV2").Int64()
				cluster.V2Upgrade.IsDoubleWriteEnabled = IsDoubleWriteEnabled
				cluster.V2Upgrade.Upgraded = Upgraded
				cluster.V2Upgrade.IsAll20XVersion = IsAll20XVersion
				cluster.V2Upgrade.ServiceCountV1 = ServiceCountV1
				cluster.V2Upgrade.InstanceCountV1 = InstanceCountV1
				cluster.V2Upgrade.ServiceCountV2 = ServiceCountV2
				cluster.V2Upgrade.InstanceCountV2 = InstanceCountV2
				cluster.V2Upgrade.SubscribeCountV2 = SubscribeCountV2
			}
			json.Unmarshal(res, &ser)
			for _, se := range ser.Doms {
				res := d.GetInstance(se, namespace.Namespace)
				var in Instance
				err := json.Unmarshal(res, &in)
				if err != nil {
					fmt.Println("json序列化错误:%s", err)
				}
				for _, host := range in.Hosts {
					metadataUrl := host.Metadata["dubbo.metadata-service.urls"]
					u, _ := regexp.Compile("pid=(.+?)&")
					_tmpmap := make([]string, 0)
					ipinfo := fmt.Sprintf("%s:%d", host.Ip, host.Port)
					_tmpmap = append(_tmpmap, namespace.NamespaceShowName, se, ipinfo, strconv.FormatBool(host.Healthy))
					if PARSEIP {
						_tmpmap = append(_tmpmap, GetHostName(host.Ip))
					} else {
						_tmpmap = append(_tmpmap, host.Ip)
					}
					_tmpmap = append(_tmpmap, fmt.Sprintf("%.1f", host.Weight))
					pid := u.FindStringSubmatch(metadataUrl)
					if len(pid) == 2 {
						_tmpmap = append(_tmpmap, pid[1])
					} else {
						_tmpmap = append(_tmpmap, "")
					}
					_tmpmap = append(_tmpmap, strconv.FormatBool(pkg.ContainerdIPCheck(host.Ip)), host.Ip, strconv.Itoa(host.Port))
					if host.Healthy {
						cluster.HealthInstance = append(cluster.HealthInstance, _tmpmap)
					} else {
						cluster.UnHealthInstance = append(cluster.UnHealthInstance, _tmpmap)
					}
					d.Clusterdata[server] = cluster
				}
			}
		}
	}
}

func GetHostName(ip string) string {
	for hostname, i := range IPDATA {
		if ip == i {
			return hostname
		}
	}
	return ip
}
