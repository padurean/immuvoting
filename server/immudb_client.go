package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"strings"
	"time"

	"github.com/codenotary/immudb/embedded/store"
	"github.com/codenotary/immudb/pkg/api/schema"
	"github.com/codenotary/immudb/pkg/auth"
	immudb_client_cache "github.com/codenotary/immudb/pkg/client/cache"
	"github.com/codenotary/immudb/pkg/client/state"
	"github.com/codenotary/immudb/pkg/database"
	immudb_logger "github.com/codenotary/immudb/pkg/logger"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

// ImmudbConfig ...
type ImmudbConfig struct {
	Address       string
	DB            string
	User          string
	Password      string
	LocalStateDir string
}

// ImmudbClient ...
type ImmudbClient struct {
	Config *ImmudbConfig

	Dial            func(string, ...grpc.DialOption) (*grpc.ClientConn, error)
	NewImmudbClient func(grpc.ClientConnInterface) schema.ImmuServiceClient
	CloseConnection func(*grpc.ClientConn) error

	StateService state.StateService

	grpcConn     *grpc.ClientConn
	immudbClient schema.ImmuServiceClient
	ctx          context.Context
}

// Init ...
func (c *ImmudbClient) Init(config *ImmudbConfig) {
	c.Config = config

	// funcs
	if c.Dial == nil {
		c.Dial = grpc.Dial
	}
	if c.NewImmudbClient == nil {
		c.NewImmudbClient = schema.NewImmuServiceClient
	}
	if c.CloseConnection == nil {
		c.CloseConnection = func(conn *grpc.ClientConn) error {
			if conn == nil {
				return nil
			}
			return conn.Close()
		}
	}
}

func (c *ImmudbClient) isConnected() bool {
	return c.grpcConn != nil && c.immudbClient != nil
}

func (c *ImmudbClient) ensureConnected(force bool) error {
	if c == nil || c.immudbClient == nil {
		return errors.New("immudb client or client wrapper is nil")
	}
	if force || !c.isConnected() {
		if err := c.Connect(); err != nil {
			return fmt.Errorf("error (re)connecting immudb client %+v: %v", c.Config, err)
		}
	}
	return nil
}

// Connect ...
func (c *ImmudbClient) Connect() error {
	if c == nil {
		return errors.New("can not connect nil immudb client")
	}

	// 0. Close previous connection (if any - check must be inside CloseConnection)
	c.CloseConnection(c.grpcConn)

	// 1. Dial to login and obtain token
	var maxSize int = 512 * 10e6
	conn, err := c.Dial(
		c.Config.Address,
		grpc.WithInsecure(),
		grpc.WithTimeout(10*time.Second),
		grpc.WithBlock(),
		grpc.WithMaxMsgSize(maxSize),
	)
	if err != nil {
		return fmt.Errorf(
			"error gRPC-dialing immudb @ %s: %v", c.Config.Address, err)
	}
	client := c.NewImmudbClient(conn)
	ctx := context.Background()
	loginReq := schema.LoginRequest{
		User:     []byte(c.Config.User),
		Password: []byte(c.Config.Password),
	}
	loginResp, err := client.Login(ctx, &loginReq)
	_ = c.CloseConnection(conn)
	if err != nil {
		return fmt.Errorf(
			"error logging in to immudb @ %s with user %s: %v",
			c.Config.Address, c.Config.User, err)
	}
	token := loginResp.GetToken()

	// 2. Dial again with the obtained token
	dialOpts := []grpc.DialOption{
		grpc.WithInsecure(),
		grpc.WithTimeout(10 * time.Second),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                2000 * time.Second,
			Timeout:             1000 * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.WithMaxMsgSize(maxSize),
	}
	conn, err = c.Dial(
		c.Config.Address,
		append(
			dialOpts,
			grpc.WithUnaryInterceptor(auth.ClientUnaryInterceptor(token)),
			grpc.WithStreamInterceptor(auth.ClientStreamInterceptor(token)))...)
	if err != nil {
		return fmt.Errorf(
			"error gRPC-redialing immudb @ %s: %v", c.Config.Address, err)
	}

	// 3. Use database
	if c.Config.DB != "" {
		client = c.NewImmudbClient(conn)
		useDBResp, err := client.UseDatabase(
			ctx, &schema.Database{Databasename: c.Config.DB})
		_ = c.CloseConnection(conn)
		if err != nil {
			return fmt.Errorf(
				"error using database %s of immudb @ %s: %v",
				c.Config.DB, c.Config.Address, err)
		}
		token = useDBResp.GetToken()

		// 4. Dial again with the obtained token (which encodes the used database)
		conn, err = c.Dial(
			c.Config.Address,
			append(
				dialOpts,
				grpc.WithMaxMsgSize(maxSize),
				grpc.WithUnaryInterceptor(auth.ClientUnaryInterceptor(token)),
				grpc.WithStreamInterceptor(auth.ClientStreamInterceptor(token)))...)
		if err != nil {
			return fmt.Errorf(
				"error gRPC-redialing immudb @ %s after use database: %v",
				c.Config.Address, err)
		}
	}

	c.ctx = ctx
	c.grpcConn = conn
	c.immudbClient = c.NewImmudbClient(conn)

	//--> set state service (!WARNING: c.immudbClient needs to be initialized BEFORE this)
	if c.StateService == nil {
		if c.Config.LocalStateDir != "" {
			if err = os.MkdirAll(c.Config.LocalStateDir, os.ModePerm); err != nil {
				return fmt.Errorf(
					"error creating local state dir %s: %v", c.Config.LocalStateDir, err)
			}
		}
		c.StateService, err = state.NewStateService(
			immudb_client_cache.NewFileCache(c.Config.LocalStateDir),
			immudb_logger.NewSimpleLoggerWithLevel(
				"cnlc-immudb-client", &NoOpWriter{}, immudb_logger.LogWarn),
			state.NewStateProvider(c.immudbClient),
			state.NewUUIDProvider(c.immudbClient))
		if err != nil {
			return fmt.Errorf("error creating immudb state (root) service: %s", err)
		}
	}
	//<--

	return nil
}

// Ping ...
func (c *ImmudbClient) Ping() error {
	return c.ensureConnected(false)
}

// Disconnect ...
func (c *ImmudbClient) Disconnect() error {
	if c == nil {
		return errors.New("can not disconnect nil immudb client")
	}
	err := c.CloseConnection(c.grpcConn)
	c.immudbClient = nil
	c.ctx = nil
	return err
}

// Errors
var (
	ErrAlreadyExists = errors.New("already exists")
	ErrNotFound      = errors.New("not found")
)

func isTokenExpired(err error) bool {
	return strings.Contains(err.Error(), "token has expired") ||
		strings.Contains(err.Error(), "please login first")
}

func (c *ImmudbClient) execute(f func() (interface{}, error)) (interface{}, error) {
	res, err := f()
	if err != nil && isTokenExpired(err) {
		log.Printf("got error '%s' => reconnecting to immudb ...", err)
		if err = c.ensureConnected(true); err == nil {
			log.Print("successfully reconnected to immudb")
			res, err = f()
		}
	}
	return res, err
}

// CreateDatabase ...
func (c *ImmudbClient) CreateDatabase(db string) error {
	if err := c.ensureConnected(false); err != nil {
		return err
	}
	sDB := &schema.Database{Databasename: db}
	if _, err := c.execute(func() (interface{}, error) {
		return c.immudbClient.CreateDatabase(c.ctx, sDB)
	}); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("%w: %v", ErrAlreadyExists, err)
		}
		return err
	}
	return nil
}

// CreateUser ...
func (c *ImmudbClient) CreateUser(
	user string,
	pass string,
	db string,
	perm uint32,
) error {
	if err := c.ensureConnected(false); err != nil {
		return err
	}
	req := &schema.CreateUserRequest{
		User:       []byte(user),
		Password:   []byte(pass),
		Permission: perm,
		Database:   db,
	}
	if _, err := c.execute(func() (interface{}, error) {
		return c.immudbClient.CreateUser(c.ctx, req)
	}); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("%w: %v", ErrAlreadyExists, err)
		}
		return err
	}
	return nil
}

// Get ...
func (c *ImmudbClient) Get(key []byte, txID uint64) ([]byte, error) {
	if err := c.ensureConnected(false); err != nil {
		return nil, err
	}
	sKey := &schema.KeyRequest{Key: key, AtTx: txID}
	item, err := c.execute(
		func() (interface{}, error) { return c.immudbClient.Get(c.ctx, sKey) })
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("key %w: %v", ErrNotFound, err)
		}
		return nil, err
	}
	return item.(*schema.Entry).Value, nil
}

// VerifiedGet ...
func (c *ImmudbClient) VerifiedGet(key []byte) (*schema.Entry, error) {
	err := c.StateService.CacheLock()
	if err != nil {
		return nil, err
	}
	defer c.StateService.CacheUnlock()

	if err := c.ensureConnected(false); err != nil {
		return nil, err
	}

	state, err := c.StateService.GetState(c.ctx, c.Config.DB)
	if err != nil {
		return nil, err
	}

	verifiableGetReq := &schema.VerifiableGetRequest{
		KeyRequest:   &schema.KeyRequest{Key: key},
		ProveSinceTx: state.TxId,
	}

	verifiableEntryItf, err := c.execute(func() (interface{}, error) {
		return c.immudbClient.VerifiableGet(c.ctx, verifiableGetReq)
	})
	if err != nil {
		if strings.Contains(err.Error(), "key not found") {
			return nil, fmt.Errorf("key %w: %v", ErrNotFound, err)
		}
		return nil, err
	}
	verifiableEntry := verifiableEntryItf.(*schema.VerifiableEntry)

	inclusionProof := schema.InclusionProofFrom(verifiableEntry.GetInclusionProof())
	dualProof := schema.DualProofFrom(verifiableEntry.GetVerifiableTx().GetDualProof())

	var eh [sha256.Size]byte

	var sourceID, targetID uint64
	var sourceAlh, targetAlh [sha256.Size]byte

	var vTx uint64
	var kv *store.KV

	if verifiableEntry.GetEntry().GetReferencedBy() == nil {
		vTx = verifiableEntry.GetEntry().GetTx()
		kv = database.EncodeKV(
			verifiableGetReq.GetKeyRequest().GetKey(),
			verifiableEntry.GetEntry().GetValue())
	} else {
		vTx = verifiableEntry.GetEntry().GetReferencedBy().GetTx()
		kv = database.EncodeReference(
			verifiableEntry.GetEntry().GetReferencedBy().GetKey(),
			verifiableEntry.GetEntry().GetKey(),
			verifiableEntry.GetEntry().GetReferencedBy().GetAtTx())
	}

	if state.TxId <= vTx {
		eh = schema.DigestFrom(
			verifiableEntry.GetVerifiableTx().GetDualProof().GetTargetTxMetadata().GetEH())

		sourceID = state.TxId
		sourceAlh = schema.DigestFrom(state.TxHash)
		targetID = vTx
		targetAlh = dualProof.TargetTxMetadata.Alh()
	} else {
		eh = schema.DigestFrom(
			verifiableEntry.GetVerifiableTx().GetDualProof().GetSourceTxMetadata().GetEH())

		sourceID = vTx
		sourceAlh = dualProof.SourceTxMetadata.Alh()
		targetID = state.TxId
		targetAlh = schema.DigestFrom(state.TxHash)
	}

	verifies := store.VerifyInclusion(
		inclusionProof,
		kv,
		eh)
	if !verifies {
		return nil, store.ErrCorruptedData
	}

	verifies = store.VerifyDualProof(
		dualProof,
		sourceID,
		targetID,
		sourceAlh,
		targetAlh,
	)
	if !verifies {
		return nil, store.ErrCorruptedData
	}

	newState := &schema.ImmutableState{
		Db:        c.Config.DB,
		TxId:      targetID,
		TxHash:    targetAlh[:],
		Signature: verifiableEntry.GetVerifiableTx().GetSignature(),
	}

	err = c.StateService.SetState(c.Config.DB, newState)
	if err != nil {
		return nil, err
	}

	return verifiableEntry.GetEntry(), nil
}

// Set ...
func (c *ImmudbClient) Set(key []byte, value []byte) error {
	if err := c.ensureConnected(false); err != nil {
		return err
	}
	kv := &schema.SetRequest{KVs: []*schema.KeyValue{{Key: key, Value: value}}}
	_, err := c.execute(
		func() (interface{}, error) { return c.immudbClient.Set(c.ctx, kv) })
	return err
}

// Reference ...
func (c *ImmudbClient) Reference(reference []byte, key []byte) error {
	if err := c.ensureConnected(false); err != nil {
		return err
	}
	ro := &schema.ReferenceRequest{ReferencedKey: key, Key: reference}
	_, err := c.execute(
		func() (interface{}, error) { return c.immudbClient.SetReference(c.ctx, ro) })
	return err
}

// Scan ...
func (c *ImmudbClient) Scan(
	prefix []byte,
	limit uint64,
	seekKey []byte,
	desc bool,
) ([]*schema.Entry, error) {
	if err := c.ensureConnected(false); err != nil {
		return nil, err
	}
	so := &schema.ScanRequest{
		Prefix:  prefix,
		Limit:   limit,
		SeekKey: seekKey,
		Desc:    desc,
		NoWait:  true,
		SinceTx: math.MaxUint64,
	}
	itemList, err := c.execute(
		func() (interface{}, error) { return c.immudbClient.Scan(c.ctx, so) })
	if err != nil {
		return nil, err
	}
	return itemList.(*schema.Entries).GetEntries(), nil
}

// Count ...
func (c *ImmudbClient) Count(prefix []byte) (uint64, error) {
	if err := c.ensureConnected(false); err != nil {
		return 0, err
	}
	kp := &schema.KeyPrefix{Prefix: prefix}
	count, err := c.execute(
		func() (interface{}, error) { return c.immudbClient.Count(c.ctx, kp) })
	if err != nil {
		return 0, err
	}
	return count.(*schema.EntryCount).GetCount(), nil
}

// CountAll returns the total number of entries
func (c *ImmudbClient) CountAll() (uint64, error) {
	if err := c.ensureConnected(false); err != nil {
		return 0, err
	}
	e := new(empty.Empty)
	currState, err := c.execute(
		func() (interface{}, error) { return c.immudbClient.CurrentState(c.ctx, e) })
	if err != nil {
		return 0, err
	}
	return currState.(*schema.ImmutableState).GetTxId(), nil
}

// ExecAll execute several commands in a transaction.
func (c *ImmudbClient) ExecAll(ops *schema.ExecAllRequest) (uint64, error) {
	if err := c.ensureConnected(false); err != nil {
		return 0, err
	}
	txMeta, err := c.execute(
		func() (interface{}, error) { return c.immudbClient.ExecAll(c.ctx, ops) })
	if err != nil {
		return 0, err
	}
	return txMeta.(*schema.TxMetadata).GetId(), nil
}

// CleanIndex cleans the index
func (c *ImmudbClient) CleanIndex() error {
	if err := c.ensureConnected(false); err != nil {
		return err
	}
	e := new(empty.Empty)
	_, err := c.execute(
		func() (interface{}, error) { return c.immudbClient.CleanIndex(c.ctx, e) })
	return err
}

// CurrentState fetches the current server state
func (c *ImmudbClient) CurrentState() (*schema.ImmutableState, error) {
	if err := c.ensureConnected(false); err != nil {
		return nil, err
	}
	e := new(empty.Empty)
	currentState, err := c.execute(
		func() (interface{}, error) { return c.immudbClient.CurrentState(c.ctx, e) })
	return currentState.(*schema.ImmutableState), err
}

// VerifiableTXByID ...
func (c *ImmudbClient) VerifiableTXByID(serverTX uint64, localTX uint64) (*schema.VerifiableTx, error) {
	if err := c.ensureConnected(false); err != nil {
		return nil, err
	}
	verifiableTX, err := c.execute(
		func() (interface{}, error) {
			return c.immudbClient.VerifiableTxById(c.ctx, &schema.VerifiableTxRequest{
				Tx:           serverTX,
				ProveSinceTx: localTX,
			})
		})
	return verifiableTX.(*schema.VerifiableTx), err
}

// History ...
func (c *ImmudbClient) History(key []byte) (*schema.Entries, error) {
	if err := c.ensureConnected(false); err != nil {
		return nil, err
	}
	entries, err := c.execute(
		func() (interface{}, error) {
			return c.immudbClient.History(c.ctx, &schema.HistoryRequest{
				Key:    key,
				Offset: 0,
				Limit:  database.MaxKeyScanLimit,
			})
		})
	return entries.(*schema.Entries), err
}
