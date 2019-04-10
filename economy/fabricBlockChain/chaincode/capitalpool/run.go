package main

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	pb "github.com/hyperledger/fabric/protos/peer"
)

//资金池合约
type Capitalpool struct {
}

type CapitalUser struct {
	ID          string  `json:"id"`          //账户唯一ID
	ALLCapital  float64 `json:"allcapital"`  //当前账户所有消费
	LeftCapital float64 `json:"leftcapital"` //当前账户余额,除了收益外
	Earning     float64 `json:"earning"`     //当前账户收益
}
type ALL struct {
	Amount float64 `json:amount`
	Price  float64 `json:price`
}

var Price float64 = 0.1
var CapitalPoolName string = "III"
var AccountBalance float64 = 1000000 //创建账户后，当前账户默认存在一定数量的token

func main() {
	err := shim.Start(new(Capitalpool))
	if err != nil {
		fmt.Printf("Error starting Capitalpool : %s", err)
	}
}

// Init initializes chaincode
// ===========================
func (t *Capitalpool) Init(stub shim.ChaincodeStubInterface) pb.Response {
	err := t.createCapitalPoolALL(stub, CapitalPoolName)
	if err != nil {
		return shim.Error(err.Error())
	}
	return shim.Success(nil)
}

// Invoke - Our entry point for InvmZcations
// ========================================
func (t *Capitalpool) Invoke(stub shim.ChaincodeStubInterface) pb.Response {
	function, args := stub.GetFunctionAndParameters()
	fmt.Println("invoke is running " + function)

	// Handle different functions
	switch function {
	case "createaccount":
		//创建账户先
		return t.CreateAccount(stub, args)
	//case "transfer":
	//	//向合约中账户充值
	//	return t.Transfer(stub,args)
	case "withdrawinside":
		//合约账户中earn提现到left账户余额中
		return t.WithdrawFromEarningtoLeft(stub, args)
	case "payment":
		//缴纳押金
		return t.payment(stub, args)
	case "balance":
		//结算工资
		return t.balance(stub, args)
	case "getglobal":
		//获取资金池数据信息
		return t.getGlobal(stub)
	case "queryuserinfo":
		//获取普通用户信息
		return t.queryUserInfo(stub, args)

	default:
		//error
		fmt.Println("invoke did not find func: " + function)
		return shim.Error("Received unknown function invocation")
	}
}

//============================
//查询普通用户相关信息
//============================
func (t *Capitalpool) queryUserInfo(stub shim.ChaincodeStubInterface, args []string) pb.Response {

	if args[0] == CapitalPoolName {
		shim.Error("No permission，please use \"getglobal\" command  try agine")
	}

	user, err := stub.GetState(args[0])
	if err != nil {
		return shim.Error(err.Error())
	}
	if user == nil {
		return shim.Error("Won't find the User " + args[0])
	}
	var User CapitalUser
	json.Unmarshal(user, &User)
	fmt.Println("User ID :			    " + User.ID)
	fmt.Println("User ALLCapital :      " + strconv.FormatFloat(User.ALLCapital, 'f', -1, 64))
	fmt.Println("User LeftCapital :     " + strconv.FormatFloat(User.LeftCapital, 'f', -1, 64))
	fmt.Println("User Earning :         " + strconv.FormatFloat(User.Earning, 'f', -1, 64))
	return shim.Success(user)
}

//=============================
//查询资金池总资金
//=============================
func (t *Capitalpool) getGlobal(stub shim.ChaincodeStubInterface) pb.Response {
	pool, err := stub.GetState(CapitalPoolName)
	if err != nil {
		return shim.Error(err.Error())
	}
	if pool == nil {
		return shim.Error("System error won't create ALL data")
	}
	Pool := new(ALL)
	json.Unmarshal(pool, Pool)
	fmt.Println("Amount: ", float64(Pool.Amount))
	fmt.Println("Price : ", Pool.Price)
	return shim.Success(pool)
}

//=============================
//资金池总创建，初始化过程
//=============================
func (t *Capitalpool) createCapitalPoolALL(stub shim.ChaincodeStubInterface, name string) error {
	all := ALL{Amount: 0, Price: Price}
	allAsBytes, _ := json.Marshal(all)
	err := stub.PutState(name, allAsBytes)
	if err != nil {
		return err
	}
	fmt.Println("Amount: ", all.Amount)
	fmt.Println("ID: ", all.Price)
	//fmt.Println("Price : ",Price)
	return nil
}

//=============================
//创建账户
//参数：ID
//=============================
func (t *Capitalpool) CreateAccount(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	if args[0] == CapitalPoolName {
		return shim.Error("This ID is already existed!!!!")
	}
	var User = CapitalUser{ID: args[0], ALLCapital: 0, LeftCapital: AccountBalance, Earning: 0}
	UserAsBytes, _ := json.Marshal(User)
	err := stub.PutState(args[0], UserAsBytes)
	if err != nil {
		return shim.Error("set UserValue error,Filed to set yajin")
	}
	return shim.Success(UserAsBytes)
}

//==============================
//向合约转账
//参数：accountID，money钱数
//==============================
func (t *Capitalpool) Transfer(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	if len(args) != 2 {
		return shim.Error("Incorrect number of arguments. Expecting 2")
	}
	if args[0] == CapitalPoolName {
		return shim.Error("No permission")
	}
	user, err := stub.GetState(args[0])
	if err != nil {
		return shim.Error(err.Error())
	}
	if user == nil {
		return shim.Error("The User " + args[0] + "isn't exist , please try to create account,use createaccount")
	}
	money, err := strconv.ParseFloat(args[1], 64)
	if err != nil {
		return shim.Error("Strconv transfer money error:	" + err.Error())
	}
	User := new(CapitalUser)
	json.Unmarshal(user, User)
	User.LeftCapital = User.LeftCapital + money
	UserAsBytes, _ := json.Marshal(User)
	err = stub.PutState(args[0], UserAsBytes)
	if err != nil {
		return shim.Error("Transfer balance to User error:	" + err.Error())
	}
	return shim.Success(nil)
}

//==============================
//从earning提现到left
//参数：accountID，money钱数
//==============================
func (t *Capitalpool) WithdrawFromEarningtoLeft(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	if len(args) != 2 {
		return shim.Error("Incorrect number of arguments. Expecting 2")
	}
	if args[0] == CapitalPoolName {
		return shim.Error("No permission")
	}
	user, err := stub.GetState(args[0])
	if err != nil {
		return shim.Error(err.Error())
	}
	if user == nil {
		return shim.Error("The User " + args[0] + "isn't exist , please try to create account,use createaccount")
	}
	money, err := strconv.ParseFloat(args[1], 64)
	if err != nil {
		return shim.Error("Strconv transfer money error:	" + err.Error())
	}
	User := new(CapitalUser)
	json.Unmarshal(user, User)
	if money < User.Earning { //提现金额少于账户中Earning部分，按具体money数量提现
		User.LeftCapital = User.LeftCapital + money
		User.Earning = User.Earning - money
	} else { //提现金额大于账户中Earning部分，直接将账户中Earning所有金额提现到Left中
		User.LeftCapital = User.LeftCapital + User.Earning
		User.Earning = 0
	}
	UserAsBytes, _ := json.Marshal(User)
	err = stub.PutState(args[0], UserAsBytes)
	if err != nil {
		return shim.Error("Withdraw from Earning to LeftCapital error:	" + err.Error())
	}
	return shim.Success(UserAsBytes)
}

// ===================================
//client 支付
//参数：	accountID  amount资源使用量
// ===================================
func (t *Capitalpool) payment(stub shim.ChaincodeStubInterface, args []string) pb.Response {

	if len(args) != 2 {
		return shim.Error("Incorrect number of arguments. Expecting 2")
	}
	if args[0] == CapitalPoolName {
		return shim.Error("NO permission!!!")
	}
	//获取消费数量，基于基本单位
	amount, err := strconv.ParseFloat(args[1], 64)
	if err != nil {
		return shim.Error("strconv jin error")
	}

	//获取资金池信息
	all, err := stub.GetState(CapitalPoolName)
	if err != nil {
		return shim.Error(err.Error())
	}
	if all == nil {
		err = t.createCapitalPoolALL(stub, CapitalPoolName)
		if err != nil {
			return shim.Error(err.Error())
		}
		all, err = stub.GetState(CapitalPoolName)
		if err != nil {
			return shim.Error(err.Error())
		}
		return shim.Error("Error get global pool err")
	}
	//格式化资金池数据
	var User CapitalUser
	All := new(ALL)
	json.Unmarshal(all, All)
	jin := float64(amount) * Price
	var allamoumt = float64(All.Amount)
	allamoumt = allamoumt + float64(jin)

	//获取账户信息，查看是否存在缴纳押金记录
	UserValue, err := stub.GetState(args[0])
	if err != nil {
		return shim.Error("get user info error," + err.Error())
	}
	if UserValue == nil { //账户未曾缴纳过押金，没有账户记录
		return shim.Error("Account isn't exist , please try to create account,use createaccount")
	} else {
		json.Unmarshal(UserValue, &User)
		if User.LeftCapital < jin {
			return shim.Error("Account havn't enough balance to pay")
		}
		//当前账户余额支出
		User.LeftCapital = User.LeftCapital - float64(jin)
		//消费记录增加
		User.ALLCapital = User.ALLCapital + float64(jin)

		UserAsBytes, _ := json.Marshal(User)
		err = stub.PutState(args[0], UserAsBytes)
		if err != nil {
			return shim.Error("update user info error")
		}
	}
	//资金池数据变更
	All.Amount = allamoumt
	AllAsBytes, _ := json.Marshal(All)
	err = stub.PutState(CapitalPoolName, AllAsBytes)
	if err != nil {
		return shim.Error("update ALL info error")
	}

	return shim.Success(nil)
}

func (t *Capitalpool) get(api shim.ChaincodeStubInterface, key string) ([]byte, error) {
	value, err := api.GetState(key)
	if err != nil {
		return nil, fmt.Errorf("Failed to get User infro : %s with error: %s", key, err)
	}
	if value == nil {
		return nil, fmt.Errorf("User not found: %s", key)
	}
	return value, nil
}

//================================
//结算过程，获取收益
//参数：	accountID   amount生产数量
//================================
func (t *Capitalpool) balance(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	if len(args) != 2 {
		return shim.Error("Incorrect number of arguments. Expecting 4")
	}
	if args[0] == CapitalPoolName {
		return shim.Error("NO permission!!!")
	}
	amount, err := strconv.ParseFloat(args[1], 64)
	if err != nil {
		return shim.Error("strconv amount error")
	}

	//获取资金池数据
	all, err := stub.GetState(CapitalPoolName)
	All := new(ALL)
	//格式化资金池数据
	json.Unmarshal(all, All)
	earning := float64(amount) * All.Price
	if float64(All.Amount) < float64(earning) {
		return shim.Error("Capital pool don't have enough maney for worker,please wait 5 mines try agin")
	}
	//资金池资金与请求资金之间校验
	var allamount = float64(All.Amount)
	allamount = allamount - float64(earning)
	if allamount < 0 {
		return shim.Error("Error capitalpool's token is less than asked")
	}
	//收益者检查
	var Toer CapitalUser
	to, err := t.get(stub, args[0])
	if err != nil {
		return shim.Error(err.Error())
	}
	if to == nil { //收益者不存在，创建对应的ID
		return shim.Error("Account isn't exist , please try to create account,use createaccount")
	} else {
		json.Unmarshal(to, &Toer)
		Toer.Earning = float64(Toer.Earning) + float64(earning)
		ToerAsBytes, _ := json.Marshal(Toer)
		err = stub.PutState(args[0], ToerAsBytes)
		if err != nil {
			return shim.Error("update user info error")
		}
	}
	//资金池资金修改
	All.Amount = allamount
	AllAsBytes, _ := json.Marshal(All)
	err = stub.PutState(CapitalPoolName, AllAsBytes)
	if err != nil {
		return shim.Error("update ALL info error")
	}
	return shim.Success(nil)
}
