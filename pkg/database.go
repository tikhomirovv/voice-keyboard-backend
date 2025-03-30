package pkg

import (
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewDatabase(config *Config) (*gorm.DB, error) {
	//options
	logMode := logger.Silent
	//debug options override
	if config.App.Debug {
		logMode = logger.Info
	}

	lg := logger.New(log.New(os.Stdout, "", log.LstdFlags), logger.Config{
		SlowThreshold:             200 * time.Millisecond,
		LogLevel:                  logMode,
		IgnoreRecordNotFoundError: false,
		Colorful:                  true,
	})

	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN: config.GetDatabaseDsnUrl(),
		//PreferSimpleProtocol: true, // disables implicit prepared statement usage
	}), &gorm.Config{
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
		Logger:                 lg,
		SkipDefaultTransaction: true,
		PrepareStmt:            config.Database.PreparedStatement,
	})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	// SetMaxIdleConns sets the maximum number of connections in the idle connection pool.
	sqlDB.SetMaxIdleConns(config.Database.MaxIdleConnections)

	// SetMaxOpenConns sets the maximum number of open connections to the database.
	sqlDB.SetMaxOpenConns(config.Database.MaxConnections)

	// SetConnMaxLifetime sets the maximum amount of time a connection may be reused.
	sqlDB.SetConnMaxLifetime(time.Duration(config.Database.MaxConnectionLifetime) * time.Second)

	// SetConnMaxIdleTime sets the maximum amount of time a connection may be idle.
	sqlDB.SetConnMaxIdleTime(time.Duration(config.Database.MaxConnectionIdleTime) * time.Second)

	return db, nil
}
