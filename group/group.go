//Package group provides functions to interact with Roblox groups.
package group

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	requests "github.com/Clan-Labs/RoGo/helpers"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/Clan-Labs/RoGo/account"
	"github.com/Clan-Labs/RoGo/auth"

	"github.com/Clan-Labs/RoGo/errs"
)

const endpoint = "https://groups.roblox.com/v1/groups/"

func (c Group) PostShout(shout string) error {
	var postShoutEndpoint = "https://groups.roblox.com/v1/groups/%v/status"
	endpoint := fmt.Sprintf(postShoutEndpoint, c.Id)
	// Check if account was provided
	if !c.BotAccount.IsAuthenticated() {
		return errors.New("this endpoint requires a valid cookie")
	}
	cookieJar, err := auth.NewJar(c.BotAccount.SecurityCookie, endpoint) // Create JAR
	if err != nil { return err }

	type reqBody struct {
		Message string `json:"message"`
	}

	reqB := reqBody{Message: shout}
	bodyJson, err := json.Marshal(reqB) // Create & marshal body
	if err != nil { return err }

	//Create req
	client := &http.Client{Timeout: 10 * time.Second, Jar: cookieJar}
	// Create an authorized request
	req, err := requests.NewAuthorizedRequest(c.BotAccount, endpoint, "PATCH", bytes.NewReader(bodyJson))
	if err != nil { return err }

	//Send Request
	res, err := client.Do(req)
	if err != nil { return err }
	defer func() { _ = res.Body.Close() }()

	// Check errors
	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden { // roblox returns 400 for unauthorized apparently?
		return errs.ErrUnauthorized
	} else if res.StatusCode == http.StatusBadRequest {
		return errs.ErrBadRequest
	}

	return nil
}

func (c Group) GetGroupRoles() ([]Role, error) {
	var endpoint = fmt.Sprintf("https://groups.roblox.com/v1/groups/%v/roles", c.Id)
	cookieJar, err := auth.NewJar(c.BotAccount.SecurityCookie, endpoint) // Create JAR
	if err != nil { return nil, err }
	client := &http.Client{Timeout: 10 * time.Second, Jar: cookieJar}

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil { return nil, err }

	res, err := client.Do(req)

	if err != nil { return nil, err }

	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return nil,
			errors.New(fmt.Sprintf("unexpected status code recieved `%v`",
				res.StatusCode))
	}

	var roles Roles
	err = json.NewDecoder(res.Body).Decode(&roles) // Decode into struct
	if err != nil { return nil, err }
	sort.Slice(roles.Roles, func(i, j int) bool { return roles.Roles[i].Rank < roles.Roles[j].Rank })
	// ^ Sort roles, roblox sometimes returns them unsorted
	return roles.Roles, nil // Return roles
}

func (c Group) GetRoleInGroup(id int) (Role, error) {
	var endpoint = fmt.Sprintf("https://groups.roblox.com/v1/users/%v/groups/roles", id)
	cookieJar, err := auth.NewJar(c.BotAccount.SecurityCookie, endpoint) // Create JAR
	if err != nil { return Role{}, err }
	client := &http.Client{Timeout: 10 * time.Second, Jar: cookieJar}

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil { return Role{}, err }

	res, err := client.Do(req)

	if err != nil { return Role{}, err }

	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return Role{},
			errors.New(fmt.Sprintf("unexpected status code recieved `%v`",
				res.StatusCode))
	}

	var jsonRes UserGroupData
	err = json.NewDecoder(res.Body).Decode(&jsonRes)
	if err != nil { return Role{}, err }

	var role Role
	for _, d := range jsonRes.Data {
		if d.RobloxGroup.Id == c.Id {
			role = d.RobloxRole
		}
	}
	return role, nil
}

func (c Group) ChangeRank(UserId int, Change int) (Role, Role, error) {
	if !c.BotAccount.IsAuthenticated() {
		return Role{}, Role{}, errors.New("this endpoint requires a valid cookie")
	}
	roles, err := c.GetGroupRoles()
	if err != nil { return Role{}, Role{}, err }
	role, err := c.GetRoleInGroup(UserId)
	if err != nil { return Role{}, Role{}, err }
	userRole := -1
	for _, r := range roles {
		userRole += 1
		if r.Id == role.Id {
			break
		}
	}
	newUserRole := userRole + Change
	if len(roles) < newUserRole || roles[newUserRole].Rank == 255 {
		return Role{}, Role{}, errs.ErrRankNotFound
	}
	err = c.SetRank(UserId, roles[newUserRole].Id)
	if err != nil { return Role{}, Role{}, err }
	return role, roles[newUserRole], nil
}

func (c Group)Promote(UserID int) (Role, Role, error) {
	if !c.BotAccount.IsAuthenticated() { return Role{}, Role{}, errs.ErrRequiresCookie }
	old, curr, err := c.ChangeRank(UserID, 1)
	if err != nil { return Role{}, Role{}, err }
	return old, curr, nil
}

func (c Group)Demote(UserID int) (Role, Role, error) {
	if !c.BotAccount.IsAuthenticated() { return Role{}, Role{}, errs.ErrRequiresCookie }
	old, curr, err := c.ChangeRank(UserID, -1)
	if err != nil { return Role{}, Role{}, err }
	return old, curr, nil
}

func (c Group) SetRank(UserID, Id int) error {
	if !c.BotAccount.IsAuthenticated() { return errs.ErrRequiresCookie }
	endpoint := fmt.Sprintf("https://groups.roblox.com/v1/groups/%v/users/%v", c.Id, UserID)
	// Check if account was provided
	if !c.BotAccount.IsAuthenticated() {
		return errs.ErrRequiresCookie
	}
	cookieJar, err := auth.NewJar(c.BotAccount.SecurityCookie, endpoint) // Create JAR

	type reqBody struct {
		RoleId int `json:"roleId"`
	}
	data := reqBody{RoleId: Id}
	bodyJson, err := json.Marshal(data) // Create & marshal body

	if err != nil { return err }

	//Create req

	client := &http.Client{Timeout: 10 * time.Second, Jar: cookieJar}
	// Create an authorized request
	req, err := requests.NewAuthorizedRequest(c.BotAccount, endpoint, "PATCH", bytes.NewReader(bodyJson))
	if err != nil { return err }

	//Send Request
	res, err := client.Do(req)
	if err != nil { return err }
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK { return errs.ErrNonOkStatus }
	return nil
}

func (c Group) GetShout() (*Shout, error) {
	//Check if unauthorized
	if c.Shout == nil { return nil, errs.ErrUnauthorized }
	return c.Shout, nil
}

//The Get function retrieves info about a Roblox group.
func Get(groupId int, acc *account.Account) (*Group, error) {

	//Make endpoint
	groupIdString := strconv.Itoa(groupId)
	URI := endpoint + groupIdString

	//Get account
	if acc == nil { acc = account.New("") }
	//Create Jar
	cookieJar, err := auth.NewJar(acc.SecurityCookie, endpoint)
	if err != nil { return nil, err }

	//Create req
	client := &http.Client{Timeout: 10 * time.Second, Jar: cookieJar}
	req, err := http.NewRequest("GET", URI, nil)
	if err != nil { return nil, err }

	//Send Request
	res, err := client.Do(req)
	if err != nil { return nil, err }
	defer func() { _ = res.Body.Close() }()

	//Check if exists
	if res.StatusCode == http.StatusBadRequest {
		return nil, errs.ErrGroupDoesntExist
	} else if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		return nil, errs.ErrUnauthorized
	}

	//Parse Response Body
	var group *Group
	err = json.NewDecoder(res.Body).Decode(&group)
	group.BotAccount = acc
	if err != nil { return nil, err }

	return group, nil
}

func (c Group) GetJoinRequests(limit ...int) (<-chan []JoinRequest, <-chan error,  error){
	URI := fmt.Sprintf("https://groups.roblox.com/v1/groups/%d/join-requests/", c.Id)
	if !c.BotAccount.IsAuthenticated() { return nil, nil, errs.ErrRequiresCookie }
	cookieJar, err := auth.NewJar(c.BotAccount.SecurityCookie, URI) // Create JAR
	if err != nil { return nil, nil, err }

	client := &http.Client{Timeout: 10 * time.Second, Jar: cookieJar}
	type EmptyBody struct {}
	jsonBody, err := json.Marshal(EmptyBody{})
	if err != nil { return nil, nil, err }

	ch := make(chan []JoinRequest, 1)
	errch := make(chan error, 1)

	type JoinRequests struct {
		Data []JoinRequest
		Cursor *string `json:"nextPageCursor"` // Pointers can be nil, and we iterate through the pages until its null
	}
	var curPage int
	var reqLimit int
	if len(limit) == 0 { reqLimit = -1
	} else { reqLimit = limit[0] }

	go func() {
		var cursor string
		for {
			if curPage == reqLimit { break }
			NewURI := URI
			if cursor != "" {
				NewURI += "?cursor="+cursor
			}
			req, err := requests.NewAuthorizedRequest(c.BotAccount, NewURI, "GET", bytes.NewReader(jsonBody))
			if err != nil { errch <- err; break }
			res, err := client.Do(req)
			if err != nil { errch <- err; break }
			if res.StatusCode != http.StatusOK {
				errch <- errors.New(fmt.Sprintf("unexpected status code '%d'", res.StatusCode))
				break
			}
			var data JoinRequests
			err = json.NewDecoder(res.Body).Decode(&data)
			if err != nil { errch <- err; break }
			for i := range data.Data { data.Data[i].Group = &c } // _, range copies the items, which means a lot of null pointers, so change by index
			ch <- data.Data
			if data.Cursor == nil { break }
			cursor = *data.Cursor
			res.Body.Close()
			curPage += 1
		}
		close(ch) // With these channels, you should be cautious to check that the channel is still open
		close(errch) // I literally spent 3 hours debugging the error channel to realise it was because the default value of an interface is nil (nil pointer dereference)
	}()
	return ch, errch, nil
}

func (c Group) Exile(UserID int) error {
	URI := fmt.Sprintf("https://groups.roblox.com/v1/groups/%d/users/%d", c.Id, UserID)
	if !c.BotAccount.IsAuthenticated() { return errs.ErrRequiresCookie }
	cookieJar, err := auth.NewJar(c.BotAccount.SecurityCookie, URI) // Create JAR
	if err != nil { return err }

	client := &http.Client{Timeout: 10 * time.Second, Jar: cookieJar}
	type EmptyBody struct {}
	jsonBody, err := json.Marshal(EmptyBody{})
	if err != nil { return err }

	req, err := requests.NewAuthorizedRequest(c.BotAccount, URI, "DELETE", bytes.NewReader(jsonBody))
	if err != nil { return err }

	res, err := client.Do(req)
	if err != nil { return err }

	defer func() { res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return errors.New(fmt.Sprintf("unexpected status code '%d'", res.StatusCode))
	}
	return nil
}

func (c Group) GetGroupPosts(limit ...int) (<-chan []GroupPost, <-chan error,  error){
	URI := fmt.Sprintf("https://groups.roblox.com/v2/groups/%v/wall/posts?limit=100&sortOrder=Desc", c.Id)
	//cookieJar, err := auth.NewJar(c.BotAccount.SecurityCookie, URI) // Create JAR
	//if err != nil { return nil, nil, err }

	client := &http.Client{Timeout: 10 * time.Second}

	ch := make(chan []GroupPost, 1)
	errch := make(chan error, 1)

	type GroupPosts struct {
		Data []GroupPost
		Cursor *string `json:"nextPageCursor"` // Pointers can be nil, and we iterate through the pages until its null
	}

	var curPage int
	var reqLimit int
	if len(limit) == 0 { reqLimit = -1
	} else { reqLimit = limit[0] }

	go func() {
		var cursor string
		for {
			if curPage == reqLimit { break }
			NewURI := URI
			if cursor != "" {
				NewURI += "?cursor="+cursor
			}
			//req, err := requests.NewAuthorizedRequest(c.BotAccount, NewURI, "GET", bytes.NewReader(jsonBody))
			req, err := http.NewRequest("GET", NewURI, bytes.NewReader([]byte{}))
			if err != nil { errch <- err; break }
			res, err := client.Do(req)
			if err != nil { errch <- err; break }
			if res.StatusCode == http.StatusForbidden {
				var errBody map[string]interface{}
				err = json.NewDecoder(res.Body).Decode(&errBody)
				if err != nil { errch <- err; return }
				errch <- errors.New(errBody["errors"].([]interface{})[0].(map[string]interface{})["userFacingMessage"].(string))
			} else if res.StatusCode != http.StatusOK {
					errch <- errors.New(fmt.Sprintf("unexpected status code '%d'", res.StatusCode))
				break
			}
			var data GroupPosts
			err = json.NewDecoder(res.Body).Decode(&data)
			if err != nil { errch <- err; break }
			for i := range data.Data { data.Data[i].Group = &c } // _, range copies the items, which means a lot of null pointers, so change by index
			ch <- data.Data
			if data.Cursor == nil { break }
			cursor = *data.Cursor
			res.Body.Close()
			curPage += 1
		}
		close(ch) // With these channels, you should be cautious to check that the channel is still open
		close(errch)
	}()
	return ch, errch, nil
}