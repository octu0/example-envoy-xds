package xds

import (
	"bytes"
	"log"
	"strconv"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/octu0/bp"

	alsv3 "github.com/envoyproxy/go-control-plane/envoy/service/accesslog/v3"
)

const (
	ACCESSLOG_LF    string = "\n"
	ACCESSLOG_DELIM string = "\t"
)

var (
	acclogBufPool = bp.NewBufferPool(10000, 1*1024)
	tzJST         = JSTLocation()
)

func JSTLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		return time.FixedZone("Asia/Tokyo", 9*60*60)
	}
	return loc
}

type AccessLog struct {
	Route                     string
	ClientAddress             string
	RemoteAddress             string
	RequestTime               time.Time
	Protocol                  string
	RequestMethod             string
	RequestPath               string
	UserAgent                 string
	Referer                   string
	ForwardedFor              string
	ResponseStatus            uint32
	RequestReceiveDuration    time.Duration // TimeToLastRxByte
	ResponseReceivingDuration time.Duration // TimeToFirstUpstreamRxByte
	ResponseCompleteDuration  time.Duration // TimeToLastUpstreamRxByte
	ClientReceivingDuration   time.Duration // TimeToFirstDownstreamTxByte
	ClientCompleteDuration    time.Duration // TimeToLastDownstreamTxByte
}

// write to buffer
func (a AccessLog) WriteTo(logId string, buf *bytes.Buffer) {
	// RequestTime as UTC
	jstRequestTime := a.RequestTime.In(tzJST)
	a.write(buf, "id:", logId)
	a.write(buf, "time:", jstRequestTime.Format("2006-01-02 15:04:05.000"))
	a.write(buf, "route:", a.Route)
	a.write(buf, "proto:", a.Protocol)
	a.write(buf, "method:", a.RequestMethod)
	a.write(buf, "status:", strconv.FormatUint(uint64(a.ResponseStatus), 10))
	a.write(buf, "path:", a.RequestPath)
	a.write(buf, "ua:", a.UserAgent)
	a.write(buf, "referer:", a.Referer)
	a.write(buf, "client:", a.ClientAddress)
	a.write(buf, "remote:", a.RemoteAddress)
	a.write(buf, "req.receive:", a.RequestReceiveDuration.String())
	a.write(buf, "res.receiving:", a.ResponseReceivingDuration.String())
	a.write(buf, "res.complete:", a.ResponseCompleteDuration.String())
	a.write(buf, "client.receiving:", a.ClientReceivingDuration.String())
	a.write(buf, "client.complete:", a.ClientCompleteDuration.String())
	buf.WriteString(ACCESSLOG_LF)
}

// make ltsv
func (a AccessLog) write(buf *bytes.Buffer, tag, value string) {
	if value == "" {
		value = "-"
	}
	buf.WriteString(tag)
	buf.WriteString(value)
	buf.WriteString(ACCESSLOG_DELIM)
}

type accesslogServiceHandler struct {
	log *log.Logger
}

func newAccesslogServiceHandler(logger *log.Logger) *accesslogServiceHandler {
	return &accesslogServiceHandler{
		log: logger,
	}
}

func (h *accesslogServiceHandler) StreamAccessLogs(stream alsv3.AccessLogService_StreamAccessLogsServer) error {
	msg, err := stream.Recv()
	if err != nil {
		h.log.Printf("error: failed to stream.Recv(): %s", err.Error())
		return err
	}

	// https://godoc.org/github.com/envoyproxy/go-control-plane/envoy/data/accesslog/v3#AccessLogCommon
	logId := msg.GetIdentifier().GetLogName()
	entries := msg.GetHttpLogs().GetLogEntry()
	logs := make([]AccessLog, len(entries))
	for i, httplog := range entries {
		props := httplog.GetCommonProperties()
		req := httplog.GetRequest()
		res := httplog.GetResponse()
		ts, _ := ptypes.Timestamp(props.GetStartTime())       // invalid date = Time{0,0}
		d1, _ := ptypes.Duration(props.GetTimeToLastRxByte()) // invalid dur = 0
		d2, _ := ptypes.Duration(props.GetTimeToFirstUpstreamRxByte())
		d3, _ := ptypes.Duration(props.GetTimeToLastUpstreamRxByte())
		d4, _ := ptypes.Duration(props.GetTimeToFirstDownstreamTxByte())
		d5, _ := ptypes.Duration(props.GetTimeToLastDownstreamTxByte())
		logs[i] = AccessLog{
			Route:                     props.GetRouteName(),
			Protocol:                  httplog.GetProtocolVersion().String(),
			ClientAddress:             props.GetDownstreamRemoteAddress().GetSocketAddress().GetAddress(),
			RemoteAddress:             props.GetUpstreamRemoteAddress().GetSocketAddress().GetAddress(),
			RequestTime:               ts,
			RequestMethod:             req.GetRequestMethod().String(),
			RequestPath:               req.GetPath(),
			UserAgent:                 req.GetUserAgent(),
			Referer:                   req.GetReferer(),
			ForwardedFor:              req.GetForwardedFor(),
			ResponseStatus:            res.GetResponseCode().GetValue(),
			RequestReceiveDuration:    d1,
			ResponseReceivingDuration: d2,
			ResponseCompleteDuration:  d3,
			ClientReceivingDuration:   d4,
			ClientCompleteDuration:    d5,
		}
	}

	for _, acclog := range logs {
		buf := acclogBufPool.Get()
		acclog.WriteTo(logId, buf)
		h.log.Writer().Write(buf.Bytes())
		acclogBufPool.Put(buf)
	}

	return nil
}
