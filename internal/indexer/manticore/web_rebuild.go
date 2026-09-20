package manticore

import "context"

func (c *Client) ResetWebSchema(ctx context.Context) error {
	if _, err := c.execSQL(ctx, "DROP TABLE IF EXISTS "+WebIndex); err != nil { return err }
	return c.EnsureSchema(ctx)
}
