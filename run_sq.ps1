$sonarqubePort = 9000
$sonarqubeImage = "sonarqube"
$sonarScannerImage = "sonarsource/sonar-scanner-cli"
$currentDir = Get-Location
$repositoryName = Split-Path -Leaf $currentDir
$goTestCoverageFile = "coverage.out"
$sonarProjectFile = "$currentDir\sonar-project.properties"
$latestCommitSHA = Invoke-Expression 'git log -1 --format=%H'

$sonarqubeContainerName = Read-Host -Prompt "Enter your Sonarqube container name (Make sure it is in 'sonar_network' network with Port 9000)"
$sonarqubeToken = Read-Host -Prompt "Enter your SonarQube user token"

$sonarProjectContent = @"
# Project identification
sonar.projectKey=$repositoryName
sonar.projectName=$repositoryName
sonar.projectVersion=1.0
sonar.web.host=sonarqube
sonar.language=go
sonar.sourceEncoding=UTF-8
sonar.sources=.
sonar.tests=.
sonar.token=$sonarqubeToken
sonar.go.tests.reportPaths=report.json
sonar.test.inclusions=**/*_test.go
sonar.go.coverage.reportPaths=coverage.out
sonar.qualitygate.wait=true
sonar.projectVersion=$latestCommitSHA
sonar.exclusions="**/*/mocks.go,mocks,**/*_mock.go,config/**/*.yaml,*.md,*.txt,Dockerfile*,goapp,report.json,coverage.out,report.xml,values.yaml,Jenkinsfile,templates/**/*.yaml,go.mod,go.sum,*.yaml,Makefile,resources/"
"@

Write-Output "INFO: Commit=${latestCommitSHA}"

$sonarProjectContent | Out-File -FilePath $sonarProjectFile -Encoding UTF8 -Force
$containerStatus = docker inspect -f '{{.State.Running}}' $sonarqubeContainerName
if ($containerStatus -eq "true") {
    Write-Output "INFO: SonarQube container is up and running."
} else {
    docker start $sonarqubeContainerName 
    if ($LASTEXITCODE -eq 0) {
        Write-Output "INFO: SonarQube container started successfully."
    } else {
        Write-Error "INFO: SonarQube container is not running"
        exit 1
    }
}
$containerIP = docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' $sonarqubeContainerName
go test ./... -coverprofile coverage.out -json > report.json
$reportJsonPath = ".\report.json"
$reportCoveragePath = ".\coverage.out"
$jsonContent = Get-Content -Raw -Path $reportJsonPath -ErrorAction Stop
function ExtractOutput {
    if ($jsonContent -match '"Action":"output".*"Output":"(.*?)"') {
        $output = $matches[1]
        Write-Output "INFO: Extracted output: $output"
    } else {
        Write-Error "INFO: Unable to extract output from report.json."
    }
}
function ReplaceEmptyStringsWithCurrentDirectory {
    $jsonContent = $jsonContent -replace '""', "`"$(Get-Location)`""
    Set-Content -Value $jsonContent -Path $reportJsonPath
    Write-Output "INFO: Empty strings replaced with current directory path in report.json."
}
ExtractOutput
ReplaceEmptyStringsWithCurrentDirectory
docker run --rm -e SONAR_HOST_URL="http://${containerIP}:${sonarqubePort}" -e SONAR_LOGIN="${sonarqubeToken}" -v "${currentDir}:/usr/src" --network sonar_network $sonarScannerImage
Remove-Item -Path $reportJsonPath
Remove-Item -Path $reportCoveragePath
Remove-Item -Path $sonarProjectFile