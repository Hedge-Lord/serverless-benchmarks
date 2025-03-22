# Define the folders containing main.go files
FOLDERS := naiveClient batchClient

# Default target
all: $(FOLDERS)

# Target to build and zip each folder
$(FOLDERS):
	@echo "Building $@..."
	cd $@ && GOOS=linux GOARCH=amd64 go build -tags lambda.norpc -o bootstrap main.go
	@echo "Zipping $@..."
	cd $@ && zip ../$@.zip bootstrap
	@echo "Cleaning up $@..."
	cd $@ && rm -f bootstrap
	@echo "$@.zip created successfully!"

# Clean up generated files
clean:
	@for folder in $(FOLDERS); do \
		rm -f $$folder.zip; \
		echo "Removed $$folder.zip"; \
	done

.PHONY: all $(FOLDERS) clean
