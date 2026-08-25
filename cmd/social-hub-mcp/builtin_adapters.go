package main

// The default binary intentionally includes a representative, bounded bundle.
// Self-hosters can add or remove blank imports and rebuild without changing the
// reusable MCP server package.
import (
	_ "social-hub/adapters/douyin"
	_ "social-hub/adapters/facebook/page"
	_ "social-hub/adapters/telegram"
	_ "social-hub/adapters/wechat/officialaccount"
	_ "social-hub/adapters/weibo"
	_ "social-hub/adapters/x"
)
