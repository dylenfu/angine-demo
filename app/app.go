package app

import "github.com/NebulousLabs/Sia/types"

// App exchange
type App struct {
}

// New app
func New() *App {
	panic("not implement yet")
}

// OnNewRound run in a sync way, we don't need to lock stateDupMtx, but stateMtx is still needed
func (app *App) OnNewRound(height, round int, block *types.Block) (interface{}, error) {
	panic("not implement yet")
}

// OnPrevote is needed
func (app *App) OnPrevote(height, round int, block *types.Block) (interface{}, error) {
	panic("not implement yet")
}

// OnCommit is needed
func (app *App) OnCommit(height, round int, block *types.Block) (interface{}, error) {
	panic("not implement yet")
}

// OnPropose is not needed
func (app *App) OnPropose(height, round int, block *types.Block) (interface{}, error) {
	panic("not implement yet")
}

// OnPrecommit is not needed
func (app *App) OnPrecommit(height, round int, block *types.Block) (interface{}, error) {
	panic("not implement yet")
}

// OnExecute one of hooks
func (app *App) OnExecute(height, round int, block *types.Block) (interface{}, error) {
	panic("not implement yet")
}
