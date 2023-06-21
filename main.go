package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"cloud.google.com/go/pubsub"
	"github.com/go-resty/resty/v2"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
	"github.com/twilio/twilio-go"
	twilioApi "github.com/twilio/twilio-go/rest/api/v2010"
)

type PubSubConfig struct {
	ProjectID      string
	TopicID        string
	SubscriptionID string
}

type TwilioConfig struct {
	AccountSID  string
	AuthToken   string
	PhoneNumber string
}

type Interviewer struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	PhoneNumber string `json:"phone_number"`
}

type Candidate struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	PhoneNumber string `json:"phone_number"`
}

type HR struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	PhoneNumber string `json:"phone_number"`
}

type Interview struct {
	ID            int    `json:"id"`
	InterviewerID int    `json:"interviewer_id"`
	CandidateID   int    `json:"candidate_id"`
	HRID          int    `json:"hr_id"`
	ScheduledTime string `json:"scheduled_time"`
	Rescheduled   bool   `json:"rescheduled"`
	InterviewLink string `json:"interview_link"`
}

var db *sql.DB

var pubsubConfig PubSubConfig
var twilioConfig TwilioConfig

func main() {

	dbConnString := "root:Sanju2001@/interview_system?charset=utf8&parseTime=True&loc=Local"
	var err error
	db, err = sql.Open("mysql", dbConnString)

	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	fmt.Println("Connected to the Database Successfully!")

	pubsubConfig = PubSubConfig{
		ProjectID:      "interviewsystem",
		TopicID:        "notification_topic",
		SubscriptionID: "notification_subscription",
	}

	twilioConfig = TwilioConfig{
		AccountSID:  "ACe2408ed74c8ebd1cbf6fc3f7dff60f20",
		AuthToken:   "32727baaed87150f0ff8423e7acdaf76",
		PhoneNumber: "+13156311532",
	}

	go startPubSubSubscriber()

	router := mux.NewRouter()

	router.HandleFunc("/interviewer/{id}", getInterviewer).Methods("GET")
	router.HandleFunc("/interviewer", createInterviewer).Methods("POST")
	router.HandleFunc("/interviewer/{id}", updateInterviewer).Methods("PUT")
	router.HandleFunc("/interviewer/{id}", deleteInterviewer).Methods("DELETE")
	router.HandleFunc("/interviewers", GetAllInterviewers).Methods("GET")

	router.HandleFunc("/candidate", createCandidate).Methods("POST")
	router.HandleFunc("/candidates", getAllCandidates).Methods("GET")
	router.HandleFunc("/candidate/{id}", getCandidate).Methods("GET")
	router.HandleFunc("/candidate/{id}", updateCandidate).Methods("PUT")
	router.HandleFunc("/candidate/{id}", deleteCandidate).Methods("DELETE")

	router.HandleFunc("/hr/{id}", getHR).Methods("GET")
	router.HandleFunc("/hr", createHR).Methods("POST")
	router.HandleFunc("/hr/{id}", updateHR).Methods("PUT")
	router.HandleFunc("/hr/{id}", deleteHR).Methods("DELETE")
	router.HandleFunc("/hrs", GetAllHRs).Methods("GET")

	router.HandleFunc("/interview/{id}", getInterview).Methods("GET")
	router.HandleFunc("/interview", createInterview).Methods("POST")
	router.HandleFunc("/interview/{id}", UpdateInterview).Methods("PUT")
	router.HandleFunc("/interview/{id}", DeleteInterview).Methods("DELETE")
	router.HandleFunc("/interviews", GetAllInterviews).Methods("GET")

	log.Fatal(http.ListenAndServe(":8000", router))
}

func createInterviewer(w http.ResponseWriter, r *http.Request) {
	var interviewer Interviewer
	err := json.NewDecoder(r.Body).Decode(&interviewer)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	existinginterviewer, err := getInterviewerByPhoneNumber(interviewer.PhoneNumber)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if existinginterviewer != nil {
		http.Error(w, "Phone number is already registered for another hr", http.StatusBadRequest)
		return
	}

	if !strings.HasPrefix(interviewer.PhoneNumber, "+91") || len(interviewer.PhoneNumber) != 13 {
		http.Error(w, "Invalid phone number format. Phone number should start with '+91' and have 10 additional digits.", http.StatusBadRequest)
		return
	}

	stmt, err := db.Prepare("INSERT INTO interviewer (id, name, phone_number) VALUES (?, ?, ?)")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(interviewer.ID, interviewer.Name, interviewer.PhoneNumber)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	SendResponse(map[string]string{"message": "Interviewer Created Successfully"}, w)

	// w.WriteHeader(http.StatusCreated)
}

func getInterviewer(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	id := params["id"]

	var interviewer Interviewer

	row := db.QueryRow("SELECT * FROM interviewer WHERE id = ?", id)
	err := row.Scan(&interviewer.ID, &interviewer.Name, &interviewer.PhoneNumber)
	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	json.NewEncoder(w).Encode(interviewer)
}

func createCandidate(w http.ResponseWriter, r *http.Request) {
	var candidate Candidate
	err := json.NewDecoder(r.Body).Decode(&candidate)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	existingCandidate, err := getCandidateByPhoneNumber(candidate.PhoneNumber)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if existingCandidate != nil {
		http.Error(w, "Phone number is already registered for another candidate", http.StatusBadRequest)
		return
	}

	if !strings.HasPrefix(candidate.PhoneNumber, "+91") || len(candidate.PhoneNumber) != 13 {
		http.Error(w, "Invalid phone number format. Phone number should start with '+91' and have 10 additional digits.", http.StatusBadRequest)
		return
	}

	stmt, err := db.Prepare("INSERT INTO candidate (id, name, phone_number) VALUES (?, ?, ?)")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(candidate.ID, candidate.Name, candidate.PhoneNumber)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	SendResponse(map[string]string{"message": "Candidate Created Successfully"}, w)
	// w.WriteHeader(http.StatusCreated)
}

func getCandidate(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	id := params["id"]

	var candidate Candidate

	row := db.QueryRow("SELECT * FROM candidate WHERE id = ?", id)
	err := row.Scan(&candidate.ID, &candidate.Name, &candidate.PhoneNumber)
	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	json.NewEncoder(w).Encode(candidate)
}

func createHR(w http.ResponseWriter, r *http.Request) {
	var hr HR
	err := json.NewDecoder(r.Body).Decode(&hr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	existinghr, err := getHRByPhoneNumber(hr.PhoneNumber)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if existinghr != nil {
		http.Error(w, "Phone number is already registered for another hr", http.StatusBadRequest)
		return
	}

	if !strings.HasPrefix(hr.PhoneNumber, "+91") || len(hr.PhoneNumber) != 13 {
		http.Error(w, "Invalid phone number format. Phone number should start with '+91' and have 10 additional digits.", http.StatusBadRequest)
		return
	}

	stmt, err := db.Prepare("INSERT INTO hr (id, name, phone_number) VALUES (?, ?, ?)")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(hr.ID, hr.Name, hr.PhoneNumber)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	SendResponse(map[string]string{"message": "HR Created Successfully"}, w)
	// w.WriteHeader(http.StatusCreated)
}

func getHR(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	id := params["id"]

	var hr HR

	row := db.QueryRow("SELECT * FROM hr WHERE id = ?", id)
	err := row.Scan(&hr.ID, &hr.Name, &hr.PhoneNumber)
	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	json.NewEncoder(w).Encode(hr)
}

func getInterview(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	id := params["id"]

	var interview Interview

	row := db.QueryRow("SELECT * FROM interview WHERE id = ?", id)
	err := row.Scan(&interview.ID, &interview.InterviewerID, &interview.CandidateID, &interview.HRID, &interview.ScheduledTime, &interview.Rescheduled, &interview.InterviewLink)
	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	json.NewEncoder(w).Encode(interview)
}

func getCandidateByPhoneNumber(phoneNumber string) (*Candidate, error) {
	query := "SELECT id, name, phone_number FROM candidate WHERE phone_number = ? LIMIT 1"

	row := db.QueryRow(query, phoneNumber)

	candidate := &Candidate{}
	err := row.Scan(&candidate.ID, &candidate.Name, &candidate.PhoneNumber)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Candidate not found
		}
		return nil, err
	}

	return candidate, nil
}

func getInterviewerByPhoneNumber(phoneNumber string) (*Interviewer, error) {
	query := "SELECT id, name, phone_number FROM interviewer WHERE phone_number = ? LIMIT 1"

	row := db.QueryRow(query, phoneNumber)

	interviewer := &Interviewer{}
	err := row.Scan(&interviewer.ID, &interviewer.Name, &interviewer.PhoneNumber)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Interviewer not found
		}
		return nil, err
	}

	return interviewer, nil
}

func getHRByPhoneNumber(phoneNumber string) (*HR, error) {
	query := "SELECT id, name, phone_number FROM hr WHERE phone_number = ? LIMIT 1"

	row := db.QueryRow(query, phoneNumber)

	hr := &HR{}
	err := row.Scan(&hr.ID, &hr.Name, &hr.PhoneNumber)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // HR not found
		}
		return nil, err
	}

	return hr, nil
}

func updateCandidate(w http.ResponseWriter, r *http.Request) {
	var candidate Candidate
	err := json.NewDecoder(r.Body).Decode(&candidate)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	stmt, err := db.Prepare("UPDATE candidate SET name = ?, phone_number = ? WHERE id = ?")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(candidate.Name, candidate.PhoneNumber, candidate.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	SendResponse(map[string]string{"message": "Candidate Details Updated Successfully"}, w)
	// w.WriteHeader(http.StatusOK)
}

func deleteCandidate(w http.ResponseWriter, r *http.Request) {
	candidateID := mux.Vars(r)["id"]

	stmt, err := db.Prepare("DELETE FROM candidate WHERE id = ?")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(candidateID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	SendResponse(map[string]string{"message": "Candidate was deleted Successfully"}, w)
	// w.WriteHeader(http.StatusOK)
}

func getAllCandidates(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT * FROM candidate")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var candidates []Candidate

	for rows.Next() {
		var candidate Candidate
		err := rows.Scan(&candidate.ID, &candidate.Name, &candidate.PhoneNumber)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		candidates = append(candidates, candidate)
	}

	err = rows.Err()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(candidates)
}

func updateHR(w http.ResponseWriter, r *http.Request) {
	var hr HR
	err := json.NewDecoder(r.Body).Decode(&hr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	stmt, err := db.Prepare("UPDATE hr SET name = ?, phone_number = ? WHERE id = ?")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(hr.Name, hr.PhoneNumber, hr.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	SendResponse(map[string]string{"message": "HR Details Updated Successfully"}, w)
	// w.WriteHeader(http.StatusOK)
}

func deleteHR(w http.ResponseWriter, r *http.Request) {
	hrID := mux.Vars(r)["id"]

	stmt, err := db.Prepare("DELETE FROM hr WHERE id = ?")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(hrID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	SendResponse(map[string]string{"message": "HR was deleted Successfully"}, w)
	// w.WriteHeader(http.StatusOK)
}

func updateInterviewer(w http.ResponseWriter, r *http.Request) {
	var interviewer Interviewer
	err := json.NewDecoder(r.Body).Decode(&interviewer)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	stmt, err := db.Prepare("UPDATE interviewer SET name = ?, phone_number = ? WHERE id = ?")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(interviewer.Name, interviewer.PhoneNumber, interviewer.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	SendResponse(map[string]string{"message": "Interviewer Details Updated Successfully"}, w)

	// w.WriteHeader(http.StatusOK)
}

func deleteInterviewer(w http.ResponseWriter, r *http.Request) {
	interviewerID := mux.Vars(r)["id"]

	stmt, err := db.Prepare("DELETE FROM interviewer WHERE id = ?")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(interviewerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	SendResponse(map[string]string{"message": "Interviewer was deleted Successfully"}, w)
	// w.WriteHeader(http.StatusOK)
}

func GetAllInterviewers(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT * FROM interviewer")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var interviewers []Interviewer

	for rows.Next() {
		var interviewer Interviewer
		err := rows.Scan(&interviewer.ID, &interviewer.Name, &interviewer.PhoneNumber)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		interviewers = append(interviewers, interviewer)
	}

	err = rows.Err()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(interviewers)
}

func GetAllHRs(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT * FROM hr")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var hrs []HR

	for rows.Next() {
		var hr HR
		err := rows.Scan(&hr.ID, &hr.Name, &hr.PhoneNumber)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		hrs = append(hrs, hr)
	}

	err = rows.Err()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(hrs)
}

func UpdateInterview(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	id := params["id"]

	var interview Interview
	var interviewer Interviewer
	var candidate Candidate
	var hr HR

	err := json.NewDecoder(r.Body).Decode(&interview)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	stmt, err := db.Prepare("UPDATE interview SET interviewer_id=?, candidate_id=?, hr_id=?, scheduled_time=?, rescheduled=?, interview_link=? WHERE id=?")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(interview.InterviewerID, interview.CandidateID, interview.HRID, interview.ScheduledTime, interview.Rescheduled, interview.InterviewLink, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	interviewer, err = GetInterviewerByID(interview.InterviewerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// err = AddOutgoingCallerID(twilioConfig.AccountSID, twilioConfig.AuthToken, interviewer.PhoneNumber)
	// if err != nil {
	// 	fmt.Println("Error:", err)
	// 	return
	// }

	// fmt.Println("Phone number added to valid caller IDs successfully.")

	// Get the HR's phone number
	hr, err = GetHRByID(interview.HRID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// err = AddOutgoingCallerID(twilioConfig.AccountSID, twilioConfig.AuthToken, hr.PhoneNumber)
	// if err != nil {
	// 	fmt.Println("Error:", err)
	// 	return
	// }

	// fmt.Println("Phone number added to valid caller IDs successfully.")

	// Get the candidate's phone number
	candidate, err = GetCandidateByID(interview.CandidateID)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = publishMessage(interviewer.PhoneNumber, "Updated interview schedule :  "+interview.ScheduledTime+" "+interview.InterviewLink)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = publishMessage(hr.PhoneNumber, "Updated interview schedule :  "+interview.ScheduledTime+" "+interview.InterviewLink)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = publishMessage(candidate.PhoneNumber, "Updated interview schedule :  "+interview.ScheduledTime+" "+interview.InterviewLink)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	SendResponse(map[string]string{"message": "Interviewer Details are Updated Successfully"}, w)
	// w.WriteHeader(http.StatusOK)
}

func DeleteInterview(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	id := params["id"]

	stmt, err := db.Prepare("DELETE FROM interview WHERE id=?")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	SendResponse(map[string]string{"message": "Interview is been deleted Successfully"}, w)
	// w.WriteHeader(http.StatusOK)
}

func GetAllInterviews(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT * FROM interview")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var interviews []Interview

	for rows.Next() {
		var interview Interview
		err := rows.Scan(&interview.ID, &interview.InterviewerID, &interview.CandidateID, &interview.HRID, &interview.ScheduledTime, &interview.Rescheduled, &interview.InterviewLink)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		interviews = append(interviews, interview)
	}

	if err = rows.Err(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(interviews)
}

func GetCandidateByID(candidateID int) (Candidate, error) {

	query := "SELECT id, name, phone_number FROM candidate WHERE id = ?"
	row := db.QueryRow(query, candidateID)

	candidate := Candidate{}
	err := row.Scan(&candidate.ID, &candidate.Name, &candidate.PhoneNumber)
	if err != nil {
		return candidate, err
	}

	return candidate, nil
}

func GetHRByID(hrID int) (HR, error) {

	query := "SELECT id, name, phone_number FROM hr WHERE id = ?"
	row := db.QueryRow(query, hrID)

	hr := HR{}
	err := row.Scan(&hr.ID, &hr.Name, &hr.PhoneNumber)
	if err != nil {
		return hr, err
	}

	return hr, nil
}

func GetInterviewerByID(interviewerID int) (Interviewer, error) {

	query := "SELECT id, name, phone_number FROM interviewer WHERE id = ?"
	row := db.QueryRow(query, interviewerID)

	interviewer := Interviewer{}
	err := row.Scan(&interviewer.ID, &interviewer.Name, &interviewer.PhoneNumber)
	if err != nil {
		return interviewer, err
	}

	return interviewer, nil
}

func createInterview(w http.ResponseWriter, r *http.Request) {
	var interview Interview
	err := json.NewDecoder(r.Body).Decode(&interview)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if interview.InterviewerID == 0 || interview.CandidateID == 0 || interview.HRID == 0 {
		http.Error(w, "Interviewer, candidate, or HR ID is missing or zero", http.StatusBadRequest)
		return
	}

	conflict, err := hasConflictingSchedule(interview.InterviewerID, interview.ScheduledTime)
	if err != nil {
		http.Error(w, "Failed to check conflicting schedules", http.StatusInternalServerError)
		return
	}
	if conflict {
		http.Error(w, "Interviewer has another interview scheduled at the same time", http.StatusBadRequest)
		return
	}

	conflict, err = hasConflictingSchedule(interview.HRID, interview.ScheduledTime)
	if err != nil {
		http.Error(w, "Failed to check conflicting schedules", http.StatusInternalServerError)
		return
	}
	if conflict {
		http.Error(w, "HR has another interview scheduled at the same time", http.StatusBadRequest)
		return
	}

	conflict, err = hasConflictingSchedule(interview.CandidateID, interview.ScheduledTime)
	if err != nil {
		http.Error(w, "Failed to check conflicting schedules", http.StatusInternalServerError)
		return
	}

	if conflict {
		http.Error(w, "Candidate has another interview scheduled at the same time", http.StatusBadRequest)
		return
	}

	stmt, err := db.Prepare("INSERT INTO interview (id, interviewer_id, candidate_id, hr_id, scheduled_time, rescheduled, interview_link) VALUES (?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(interview.ID, interview.InterviewerID, interview.CandidateID, interview.HRID, interview.ScheduledTime, interview.Rescheduled, interview.InterviewLink)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	interviewer, err := GetInterviewerByID(interview.InterviewerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// err = AddOutgoingCallerID(twilioConfig.AccountSID, twilioConfig.AuthToken, interviewer.PhoneNumber)
	// if err != nil {
	// 	fmt.Println("Error:", err)
	// 	return
	// }

	// fmt.Println("Phone number added to valid caller IDs successfully.")

	// Get the HR's phone number
	hr, err := GetHRByID(interview.HRID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// err = AddOutgoingCallerID(twilioConfig.AccountSID, twilioConfig.AuthToken, hr.PhoneNumber)
	// if err != nil {
	// 	fmt.Println("Error:", err)
	// 	return
	// }

	// fmt.Println("Phone number added to valid caller IDs successfully.")

	// Get the candidate's phone number
	candidate, err := GetCandidateByID(interview.CandidateID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// err = AddOutgoingCallerID(twilioConfig.AccountSID, twilioConfig.AuthToken, hr.PhoneNumber)
	// if err != nil {
	// 	fmt.Println("Error:", err)
	// 	return
	// }

	// fmt.Println("Phone number added to valid caller IDs successfully.")

	interviewLink := generateInterviewLink()
	fmt.Println("Random interview link:", interviewLink)

	err = publishMessage(interviewer.PhoneNumber, "You have an interview scheduled at "+interview.ScheduledTime+" "+interview.InterviewLink)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = publishMessage(hr.PhoneNumber, "You have an interview scheduled at "+interview.ScheduledTime+" "+interview.InterviewLink)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = publishMessage(candidate.PhoneNumber, "You have an interview scheduled at "+interview.ScheduledTime+" "+interview.InterviewLink)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	SendResponse(map[string]string{"message": "Interview Created Successfully"}, w)
	// w.WriteHeader(http.StatusCreated)
}

func publishMessage(phoneNumber, message string) error {

	ctx := context.Background()

	client, err := pubsub.NewClient(ctx, pubsubConfig.ProjectID)
	if err != nil {
		return err
	}

	topic := client.Topic(pubsubConfig.TopicID)

	msg := &pubsub.Message{
		Data: []byte(fmt.Sprintf(`{"phone_number": "%s", "message": "%s"}`, phoneNumber, message)),
	}

	_, err = topic.Publish(ctx, msg).Get(ctx)
	if err != nil {
		return err
	}

	return nil
}

func startPubSubSubscriber() {
	ctx := context.Background()

	credentialsPath := "/Users/sanjanajadhav/Downloads/key.json"

	err := os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", credentialsPath)
	if err != nil {
		log.Fatal("Failed to set GOOGLE_APPLICATION_CREDENTIALS:", err)
	}

	client, err := pubsub.NewClient(ctx, pubsubConfig.ProjectID)
	if err != nil {
		log.Fatal(err)
	}

	sub := client.Subscription(pubsubConfig.SubscriptionID)
	err = sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
		// Retrieve the phone number and message from the Pub/Sub message
		var pubsubMessage struct {
			PhoneNumber string `json:"phone_number"`
			Message     string `json:"message"`
		}

		err := json.Unmarshal(msg.Data, &pubsubMessage)
		if err != nil {
			log.Println("Failed to unmarshal Pub/Sub message:", err)
			msg.Ack()
			return
		}

		err = sendSMS(pubsubMessage.PhoneNumber, pubsubMessage.Message)
		if err != nil {
			log.Println("Failed to send SMS:", err)
			msg.Ack()
			return
		}

		// Acknowledge the message
		msg.Ack()
	})
	if err != nil {
		log.Fatal(err)
	}
}

func sendSMS(phoneNumber, message string) error {
	client := twilio.NewRestClientWithParams(twilio.ClientParams{
		Username: twilioConfig.AccountSID,
		Password: twilioConfig.AuthToken,
	})

	params := &twilioApi.CreateMessageParams{
		From: &twilioConfig.PhoneNumber,
		To:   &phoneNumber,
		Body: &message,
	}

	_, err := client.Api.CreateMessage(params)
	if err != nil {
		return err
	}
	return nil
}

func hasConflictingSchedule(userID int, scheduledTime string) (bool, error) {
	db, err := sql.Open("mysql", "root:Sanju2001@/interview_system?charset=utf8&parseTime=True&loc=Local")
	if err != nil {
		return false, err
	}
	defer db.Close()

	query := "SELECT COUNT(*) FROM interview WHERE (interviewer_id = ? OR hr_id = ? OR candidate_id = ?) AND scheduled_time = ?"
	stmt, err := db.Prepare(query)
	if err != nil {
		return false, err
	}
	defer stmt.Close()

	var count int
	err = stmt.QueryRow(userID, userID, userID, scheduledTime).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func AddOutgoingCallerID(accountSid, authToken, phoneNumber string) error {

	client := resty.New()

	client.SetBasicAuth(accountSid, authToken)

	apiURL := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/OutgoingCallerIds.json", accountSid)

	payload := map[string]string{
		"PhoneNumber": phoneNumber,
	}

	resp, err := client.R().
		SetFormData(payload).
		Post(apiURL)

	if err != nil {
		fmt.Println("Error:", err)
		return err
	}

	if resp.StatusCode() >= 200 && resp.StatusCode() < 300 {
		return nil
	} else {
		return err
	}

}

func generateInterviewLink() string {
	linkLength := 20
	randomBytes := make([]byte, linkLength)
	_, err := rand.Read(randomBytes)
	if err != nil {
		panic(err)
	}
	interviewLink := base64.URLEncoding.EncodeToString(randomBytes)
	interviewLink = interviewLink[:linkLength]
	interviewLink = "https://" + interviewLink
	return interviewLink
}

func SendResponse(v any, w http.ResponseWriter) {
	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
