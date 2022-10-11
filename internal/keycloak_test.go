package internal_test

import (
	"bufio"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/reddec/keycloak-ext-operator/internal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKeycloak_Authorize(t *testing.T) {
	realm := os.Getenv("REALM")
	ctx := context.TODO()
	k, err := internal.FromEnv()
	require.NoError(t, err)

	client := k.Authorize(ctx)
	require.NoError(t, client.Error())
	require.NotEmpty(t, client)

	list, err := client.Clients(ctx, realm).All()
	require.NoError(t, err)
	require.NotEmpty(t, list)
	t.Log(list)

	const clientID = "demo.example.com"
	if info, err := client.Clients(ctx, realm).Find(clientID); err == nil {
		err = client.Delete(ctx, realm, info.ID)
		require.NoError(t, err)
	}

	draft := internal.Generate("demo.example.com")
	draft.Name = "test/" + draft.Name
	draft.ID = "abcdefd"
	id, err := client.Create(ctx, realm, draft)
	require.NoError(t, err)
	require.NotEmpty(t, id)
	t.Log(draft)
	t.Log(id)

	one, err := client.Get(ctx, realm, draft.ID)
	require.NoError(t, err)
	require.NotEmpty(t, one)
	require.NotEmpty(t, one.Secret)

	_, notFoundErr := client.Get(ctx, realm, "UNKNOWNID")
	require.ErrorIs(t, notFoundErr, internal.ErrClientNotFound)

	info, err := client.Clients(ctx, realm).Find("demo.example.com")
	require.NoError(t, err)
	t.Logf("%+v", info)

	err = client.Update(ctx, info.ID, realm, internal.ClientDraft{
		RootURL: "https://not-demo.example.com",
	})
	require.NoError(t, err)

	newInfo, err := client.Clients(ctx, realm).Find("demo.example.com")
	require.NoError(t, err)
	assert.Equal(t, "https://not-demo.example.com", newInfo.RootURL)

	newInfo.RootURL = info.RootURL
	assert.Equal(t, info, newInfo)
}

func TestMain(m *testing.M) {
	f, err := os.Open("../.env")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		k, v, _ := strings.Cut(line, "=")
		if err := os.Setenv(k, v); err != nil {
			panic(err)
		}
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}
	_ = f.Close()
	if code := m.Run(); code != 0 {
		panic(code)
	}
}
