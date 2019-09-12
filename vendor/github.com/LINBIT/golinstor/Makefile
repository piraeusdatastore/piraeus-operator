all:
	echo "The only meaningful target is 'apiconsts'"

gitclean:
	git diff-index --quiet --exit-code HEAD

.PHONY: apiconsts
apiconsts: gitclean
	git submodule update --remote
	go generate
	git commit -am "apiconsts: submodule pull"
