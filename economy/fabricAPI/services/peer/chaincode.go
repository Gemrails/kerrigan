package peer

import (
	"os/exec"
)

func RunCmd(cca string, params string) (string, error) {
	docker, err := exec.LookPath("docker")
	if err != nil {
		return "not find docker", err
	}

	cmd := exec.Command(docker, "exec", "-i", "cli", "peer", "chaincode", "invoke", "-n", cca, "-c", params, "-C", "myc")
	cmd.Start()
	return "", nil
	//data := strings.Split(string(output[:]), "payload:")
	//if len(data) == 2 {
	//	str := strings.Replace(data[1], "\\", "", -1)
	//	if len(str) > 2 {
	//		str = str[1 : len(str)-1]
	//	}
	//	return str, nil
	//}
	//
	//data = strings.Split(string(output[:]), "message:")
	//return "", errors.New(data[1])
}
