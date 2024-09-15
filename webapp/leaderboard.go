// dns-server-roleplay/webapp/leaderboard.go
package main

import (
	"context"
	"log"
	"sort"
)

type PlayerScore struct {
	PlayerID string
	Score    int64
}

func updateLeaderboard(playerID, action string) {
	ctx := context.Background()
	var delta int64
	if action == "correct" {
		delta = 1
	} else {
		delta = -1
	}

	err := rdb.HIncrBy(ctx, "leaderboard", playerID, delta).Err()
	if err != nil {
		log.Println("Error updating leaderboard:", err)
	}
}

func getLeaderboard() []PlayerScore {
	ctx := context.Background()
	scores, err := rdb.HGetAll(ctx, "leaderboard").Result()
	if err != nil {
		log.Println("Error retrieving leaderboard:", err)
		return nil
	}

	var leaderboard []PlayerScore
	for playerID, scoreStr := range scores {
		score, err := rdb.HGet(ctx, "leaderboard", playerID).Int64()
		if err != nil {
			continue
		}
		leaderboard = append(leaderboard, PlayerScore{
			PlayerID: playerID,
			Score:    score,
		})
	}

	// Sort leaderboard
	sort.Slice(leaderboard, func(i, j int) bool {
		return leaderboard[i].Score > leaderboard[j].Score
	})

	return leaderboard
}
