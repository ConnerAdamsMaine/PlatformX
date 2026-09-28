package controller

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/404cnf/meshgrid/internal/core"
)

type Server struct { Store *core.Store; Identity core.Identity; Token string }

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { write(w, 200, map[string]any{"ok":true,"peerId":s.Identity.ID}) })
	mux.HandleFunc("GET /v1/status", s.auth(s.status))
	mux.HandleFunc("POST /v1/heartbeat", s.auth(s.heartbeat))
	mux.HandleFunc("POST /v1/workloads", s.auth(s.createWorkload))
	mux.HandleFunc("DELETE /v1/workloads/{id}", s.auth(s.deleteWorkload))
	mux.HandleFunc("GET /v1/workloads", s.auth(s.listWorkloads))
	mux.HandleFunc("GET /v1/nodes", s.auth(s.listNodes))
	mux.HandleFunc("POST /v1/peers", s.auth(s.addPeer))
	mux.HandleFunc("GET /v1/identity", s.auth(func(w http.ResponseWriter, _ *http.Request){ write(w,200,map[string]string{"id":s.Identity.ID,"publicKey":s.Identity.PublicKey}) }))
	return logging(mux)
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc { return func(w http.ResponseWriter, r *http.Request) {
	got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if subtle.ConstantTimeCompare([]byte(got), []byte(s.Token)) != 1 { write(w, 401, map[string]string{"error":"unauthorized"}); return }
	next(w,r)
} }

func (s *Server) status(w http.ResponseWriter, _ *http.Request) { var out core.State; _ = s.Store.View(func(st core.State) error { out=st; return nil }); write(w,200,out) }

func (s *Server) heartbeat(w http.ResponseWriter, r *http.Request) {
	var hb core.Heartbeat; if err := decode(r,&hb); err != nil || hb.Node.ID=="" { write(w,400,map[string]string{"error":"invalid heartbeat"}); return }
	now:=time.Now().UTC(); hb.Node.LastSeen=now
	err:=s.Store.Update(func(st *core.State) error {
		for _, report := range hb.Assignments {
			if old,ok:=st.Assignments[report.ID]; ok && old.NodeID==hb.Node.ID {
				if old.State == "deleted" && report.State == "deleted" { delete(st.Assignments, report.ID); continue }
				report.UpdatedAt=now; st.Assignments[report.ID]=report
			}
		}
		st.Nodes[hb.Node.ID]=hb.Node; core.Reconcile(st,now); return nil
	})
	if err != nil { write(w,500,map[string]string{"error":err.Error()}); return }
	var reply core.AgentReply
	_ = s.Store.View(func(st core.State) error { for _,a:=range st.Assignments { if a.NodeID==hb.Node.ID { if wl,ok:=st.Workloads[a.WorkloadID];ok { reply.Assignments=append(reply.Assignments,core.AssignmentView{Assignment:a,Workload:wl.Spec}) } } }; return nil })
	write(w,200,reply)
}

func (s *Server) createWorkload(w http.ResponseWriter,r *http.Request) {
	var spec core.WorkloadSpec; if err:=decode(r,&spec);err!=nil { write(w,400,map[string]string{"error":err.Error()});return }
	if err:=validateSpec(spec);err!=nil { write(w,400,map[string]string{"error":err.Error()});return }
	now:=time.Now().UTC(); id:=fmt.Sprintf("w-%d",now.UnixNano()); wl:=core.Workload{ID:id,Owner:s.Identity.ID,Spec:spec,CreatedAt:now}
	if err:=s.Store.Update(func(st *core.State)error{st.Workloads[id]=wl;core.Reconcile(st,now);return nil});err!=nil { write(w,500,map[string]string{"error":err.Error()});return }
	write(w,201,wl)
}

func (s *Server) deleteWorkload(w http.ResponseWriter,r *http.Request) { id:=r.PathValue("id"); err:=s.Store.Update(func(st *core.State)error{if _,ok:=st.Workloads[id];!ok{return errors.New("not found")};delete(st.Workloads,id);for k,a:=range st.Assignments{if a.WorkloadID==id{a.State="deleted";st.Assignments[k]=a}};return nil});if err!=nil{write(w,404,map[string]string{"error":err.Error()});return};write(w,200,map[string]bool{"deleted":true}) }

func (s *Server) listWorkloads(w http.ResponseWriter,_ *http.Request){ var out []core.Workload;_ = s.Store.View(func(st core.State)error{for _,x:=range st.Workloads{out=append(out,x)};return nil});write(w,200,out) }
func (s *Server) listNodes(w http.ResponseWriter,_ *http.Request){ var out []core.Node;_ = s.Store.View(func(st core.State)error{for _,x:=range st.Nodes{out=append(out,x)};return nil});write(w,200,out) }
func (s *Server) addPeer(w http.ResponseWriter,r *http.Request){var p core.Peer;if err:=decode(r,&p);err!=nil||p.ID==""||p.URL==""||p.PublicKey==""{write(w,400,map[string]string{"error":"id, url and publicKey required"});return};p.Enabled=true;_ = s.Store.Update(func(st *core.State)error{st.Peers[p.ID]=p;return nil});write(w,201,p)}

func validateSpec(s core.WorkloadSpec) error { if s.Name==""||s.Image==""{return errors.New("name and image required")};if s.Replicas<1||s.Replicas>1000{return errors.New("replicas must be 1..1000")};if s.Resources.CPUMillis<0||s.Resources.MemoryMB<0||s.Resources.GPUs<0{return errors.New("resources cannot be negative")};if s.Restart==""{s.Restart="unless-stopped"};return nil }
func decode(r *http.Request,v any)error{defer r.Body.Close();d:=json.NewDecoder(http.MaxBytesReader(nil,r.Body,1<<20));d.DisallowUnknownFields();return d.Decode(v)}
func write(w http.ResponseWriter,status int,v any){w.Header().Set("Content-Type","application/json");w.WriteHeader(status);_ = json.NewEncoder(w).Encode(v)}
func logging(next http.Handler)http.Handler{return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){start:=time.Now();next.ServeHTTP(w,r);log.Printf("%s %s %s",r.Method,r.URL.Path,time.Since(start))})}
