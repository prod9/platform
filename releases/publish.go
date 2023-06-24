package releases

// TODO: Publish git tags
// TODO: Publish directly to docker, for testing
// func ToRegistry(ctx context.Context, job *builder.Job, c *dagger.Container) error {
// 	if hash, err := c.Publish(ctx, job.ImageName); err != nil {
// 		return err
// 	} else {
// 		log.Println("published", hash)
// 		return nil
// 	}
// }

// func ToFile(ctx context.Context, job *builder.Job, c *dagger.Container) error {
// 	outname := job.Name + ".docker"

// 	if hash, err := c.Export(ctx, outname); err != nil {
// 		return err

// 	} else {
// 		log.Println("exported", hash)
// 		log.Println("run `docker load -i " + outname + "` to load into Docker")
// 		return nil
// 	}
// }
