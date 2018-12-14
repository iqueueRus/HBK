package main

import (
	"encoding/json"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/mgo.v2/bson"
	_ "html/template"
	"log"
	"net/http"
	"time"
)

func toJSON(s string) (map[string]interface{}, bool) {
	var js map[string]interface{}
	isJson := json.Unmarshal([]byte(s), &js) == nil
	if !isJson {
		return nil, false
	}
	return js, true
}

// Login Handler Function
// Gets posted username and password. If the user is registered
// and the password is correct, it sends response with json web token,
// logins and directs to dashboard.
func loginHandler(w http.ResponseWriter, r *http.Request) {
	var user UserCredentials

	//decode request into UserCredentials struct
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, "Error in request")
		return
	}

	session := connect()
	defer session.Close()
	result := User{}
	collection := session.DB("godict").C("users")
	err = collection.Find(bson.M{"username": user.Username}).One(&result)
	if err != nil {
		JsonResponse("user doesn't exist", w)
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(result.Password), []byte(user.Password))

	if err != nil {
		JsonResponse("password is not true", w)
		return
	}

	//create a rsa 256 signer
	signer := jwt.New(jwt.GetSigningMethod("RS256"))
	claims := make(jwt.MapClaims)
	//set claims
	claims["iss"] = "admin"
	claims["exp"] = time.Now().Add(time.Minute * 60).Unix()
	claims["CustomUserInfo"] = struct {
		Name string
		Role string
	}{user.Username, "Member"}
	signer.Claims = claims
	tokenString, err := signer.SignedString(SignKey)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, "Error while signing the token")
		log.Printf("Error signing token: %v\n", err)
	}

	// create a user-token pair

	collection = session.DB("godict").C("usertoken")
	newPair := TokenUserPair{User: result, Token: tokenString, Timestamp: bson.Now()}
	err = collection.Insert(newPair)

	if err != nil {
		http.Error(w, "everything is something happened: "+err.Error(), http.StatusBadRequest)
		return
	}

	// return token string
	JsonResponse(tokenString, w)

}

// Register Handler Function
// Gets posted username and password. If the username is not registered
// before, registers user and directs to login page.
func registerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		JsonResponse("wrong method", w)
		return
	}
	var user UserCredentials

	//decode request into UserCredentials struct
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, "Error in request")
		return
	}

	username := user.Username
	password := user.Password

	session := connect()
	defer session.Close()

	collection := session.DB("godict").C("users")

	result := User{}

	err = collection.Find(bson.M{"username": username}).Select(bson.M{"username": username}).One(&result)

	if err == nil {
		JsonResponse("already registered username", w)
		return
	}

	createdAt := time.Now()

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	checkInternalServerError(err, w)

	newUser := User{Username: username, Password: string(hashedPassword), Datetime: createdAt}
	err = collection.Insert(newUser)

	if err != nil {
		JsonResponse("db error", w)
		return
	}
	JsonResponse("success", w)
}

// Gets posted entry, encrypts it with the provided key and saves to DB.
func postHandler(w http.ResponseWriter, r *http.Request) {
	var postedJSON ThreeWayStruct

	//decode request into UserCredentials struct
	err := json.NewDecoder(r.Body).Decode(&postedJSON)
	if err != nil {
		JsonResponse("error in request", w)
		return
	}

	uncryptedText := postedJSON.One
	titleOfEntry := postedJSON.Three

	payload, test := toJSON(uncryptedText)

	if test != true {
		JsonResponse("JSON parse error", w)
		return
	}

	token := w.Header().Get("token")

	// get user by token
	user := getUserByToken(token)

	createdAt := time.Now()

	session := connect()
	defer session.Close()

	collection := session.DB("godict").C("entries")

	newEntry := Entry{User: user, Title: titleOfEntry, Day: createdAt, Updated: createdAt, Payload: payload}
	err = collection.Insert(newEntry)

	if err != nil {
		JsonResponse("db error", w)
		return
	}
	JsonResponse("success", w)
}

// Sorts and returns all encrypted entries posted by user.
func listHandler(w http.ResponseWriter, r *http.Request) {

	token := w.Header().Get("token")

	// get user by token
	user := getUserByToken(token)

	results := []Entry{}

	session := connect()
	defer session.Close()

	collection := session.DB("godict").C("entries")
	err := collection.Find(bson.M{"user._id": user.Id}).Sort("-day").All(&results)

	if err != nil {
		http.Error(w, "error: "+err.Error(), http.StatusBadRequest)
		return
	}

	JsonResponse(results, w)
}

// Returns specified entry.
func entryHandler(w http.ResponseWriter, r *http.Request) {
	var postedJSON OneWayStruct

	//decode request into UserCredentials struct
	err := json.NewDecoder(r.Body).Decode(&postedJSON)
	if err != nil {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, "Error in request")
		return
	}

	entryId := postedJSON.One

	token := w.Header().Get("token")

	usr := getUserByToken(token)
	ent := getEntryById(entryId)

	if usr.Id == ent.User.Id {
		JsonResponse(ent, w)
	}

}

// Decrypts posted text with posted key.
func decryptHandler(w http.ResponseWriter, r *http.Request) {
	var postedJSON TwoWayStruct
	err := json.NewDecoder(r.Body).Decode(&postedJSON)
	if err != nil {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, "Error in request")
		return
	}
	encryptedText := postedJSON.One
	unhashedKey := postedJSON.Two

	hashedKey := hash(unhashedKey)
	decryptedText := decrypt(hashedKey, encryptedText)

	JsonResponse(decryptedText, w)
}

// Deletes the entry with posted id.
// The user for the posted token should be same
// with the one that's binded into entry and since
// keys are not stored in DB, this is verified with password.
func deleteHandler(w http.ResponseWriter, r *http.Request) {
	var postedJSON TwoWayStruct
	err := json.NewDecoder(r.Body).Decode(&postedJSON)
	if err != nil {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, "Error in request")
		return
	}

	entryId := postedJSON.One
	pass := postedJSON.Two

	token := w.Header().Get("token")

	ent := getEntryById(entryId)
	usr := getUserByToken(token)

	err = bcrypt.CompareHashAndPassword([]byte(usr.Password), []byte(pass))

	if err != nil {
		JsonResponse("password is not true", w)
		return
	}

	if usr.Id == ent.User.Id {
		getById := bson.M{"_id": bson.ObjectIdHex(entryId)}

		session := connect()
		defer session.Close()

		collection := session.DB("godict").C("entries")
		err = collection.Remove(getById)

		if err != nil {
			JsonResponse("everything is something happened", w)
		} else {
			JsonResponse("success", w)
		}
	}
}

// Edits the entry with posted id.
// Replace the content with posted content after encryption.
// The user for the posted token should be same
// with the one that's binded into entry. Stored tokens are
// used to compare to verify.
func editHandler(w http.ResponseWriter, r *http.Request) {
	var postedJSON FourWayStruct
	err := json.NewDecoder(r.Body).Decode(&postedJSON)
	if err != nil {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, "Error in request")
		return
	}

	newContent := postedJSON.One
	entryId := postedJSON.Two
	titleOfEntry := postedJSON.Four

	token := w.Header().Get("token")

	usr := getUserByToken(token)
	ent := getEntryById(entryId)

	if usr.Id == ent.User.Id {

		getById := bson.M{"_id": ent.Id}

		payload, test := toJSON(newContent)

		if test != true {
			JsonResponse("JSON parse error", w)
			return
		}

		change := bson.M{"$set": bson.M{"payload": payload, "title": titleOfEntry, "updated": time.Now()}}

		session := connect()
		defer session.Close()

		collection := session.DB("godict").C("entries")
		err = collection.Update(getById, change)

		if err != nil {
			JsonResponse("everything is something happened", w)
		} else {
			JsonResponse("success", w)
		}
	}

}

// Gets the user by the token and change its password.
// Old password is verified before changing to new one.
func changePassword(w http.ResponseWriter, r *http.Request) {
	var postedJSON TwoWayStruct
	err := json.NewDecoder(r.Body).Decode(&postedJSON)
	if err != nil {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, "Error in request")
		return
	}

	token := w.Header().Get("token")
	usr := getUserByToken(token)
	oldpass := postedJSON.One
	newpass := postedJSON.Two

	err = bcrypt.CompareHashAndPassword([]byte(usr.Password), []byte(oldpass))

	if err != nil {
		JsonResponse("password is not true", w)
		return
	}

	getById := bson.M{"_id": usr.Id}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newpass), bcrypt.DefaultCost)

	checkInternalServerError(err, w)

	change := bson.M{"$set": bson.M{"password": hashedPassword}}

	session := connect()
	defer session.Close()

	collection := session.DB("godict").C("users")
	err = collection.Update(getById, change)

	if err != nil {
		JsonResponse("everything is something happened", w)
	} else {
		JsonResponse("success", w)
	}

}

// Gets the user by the token and delete the user if posted password is true.
func deleteUser(w http.ResponseWriter, r *http.Request) {
	var postedJSON OneWayStruct
	err := json.NewDecoder(r.Body).Decode(&postedJSON)
	if err != nil {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, "Error in request")
		return
	}

	token := w.Header().Get("token")
	usr := getUserByToken(token)
	pass := postedJSON.One
	err = bcrypt.CompareHashAndPassword([]byte(usr.Password), []byte(pass))

	if err != nil {
		JsonResponse("password is not true", w)
		return
	}

	getById := bson.M{"_id": usr.Id}

	session := connect()
	defer session.Close()

	collection := session.DB("godict").C("users")
	err = collection.Remove(getById)

	if err != nil {
		JsonResponse("everything is something happened", w)
	} else {
		JsonResponse("success", w)
	}
}
